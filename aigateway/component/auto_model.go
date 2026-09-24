package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/semanticrouter"
	"opencsg.com/csghub-server/common/config"
)

const (
	// maxAutoRouteMessages mirrors the ranking service's own limit.  Longer
	// conversations are trimmed to the most recent turns rather than being
	// rejected outright.
	maxAutoRouteMessages = 2000

	defaultAutoModelID   = "auto"
	defaultAutoTimeout   = 5 * time.Second
	defaultAutoHealthTTL = 5 * time.Minute
)

// These codes are the ones the plan layer maps to HTTP statuses.  They
// must stay in step with categorizePlanError in aigateway/handler/plan.
const (
	// autoRouteCodeUnavailable marks a failure of the ranking service
	// itself, which is temporary and worth retrying.
	autoRouteCodeUnavailable = "model_unavailable"
	// autoRouteCodeNotFound marks a request that could not be resolved to
	// a model even though the ranking service answered.  Retrying the
	// same request will not help.
	autoRouteCodeNotFound = "model_not_found"
	// autoRouteCodeRequiredUpstream marks a pinned conversation whose
	// upstream can no longer serve it.  It is the same code the ordinary
	// resolution path raises, so each protocol renders the answer it
	// already renders for that condition rather than a second one
	// invented here.
	autoRouteCodeRequiredUpstream = "required_upstream_unavailable"
)

// autoRouteError carries the model error code so a ranking service that
// is merely down is reported as a temporary unavailability rather than as
// a model that does not exist.  It satisfies the plan layer's CodedError
// without the two packages depending on each other.
type autoRouteError struct {
	code string
	err  error
}

func newAutoRouteError(code string, format string, args ...any) *autoRouteError {
	return &autoRouteError{code: code, err: fmt.Errorf(format, args...)}
}

func (e *autoRouteError) Error() string          { return e.err.Error() }
func (e *autoRouteError) Unwrap() error          { return e.err }
func (e *autoRouteError) ModelErrorCode() string { return e.code }

// AutoModelRouter backs the virtual "auto" model.  It answers two
// questions the rest of the gateway needs: whether automatic routing is
// currently usable at all, and — for one turn — which of the platform's
// real models should serve it.
//
// It deliberately takes the candidate models as an argument rather than
// loading them itself, so it stays free of any dependency on the model
// catalogue and can be composed by whoever already has the caller's
// visible models in hand.
type AutoModelRouter interface {
	// ModelID is the virtual model ID clients request to opt in.
	ModelID() string
	// Available reports whether the ranking service is answering.  The
	// result is cached for the configured TTL, so this is cheap enough to
	// call on the model-listing path.
	Available(ctx context.Context) bool
	// Select ranks candidates for the turn and returns the highest-ranked
	// one that the caller can actually use.  It returns an error when the
	// ranking service fails or when no candidate matches, so the request
	// fails loudly instead of silently landing on an arbitrary model.
	Select(ctx context.Context, input types.AutoRouteInput, candidates []types.Model) (*types.AutoRouteDecision, error)
}

type autoModelRouter struct {
	client    semanticrouter.Client
	modelID   string
	healthTTL time.Duration

	// mu guards the cached health state.  It is never held across the
	// network call, so a slow or hung ranking service cannot stall the
	// callers reading the cached answer.
	mu          sync.Mutex
	healthy     bool
	probed      bool
	probing     bool
	lastProbeAt time.Time
	// generation advances on every committed health decision.  A probe
	// carries the generation it started from and discards its own answer
	// if anything decided otherwise while it was in flight, so a stale
	// probe cannot overwrite a newer, more definitive result.
	generation uint64
	// indexVersion is what the service last reported it ranks against.
	// The ordinary ranking response does not carry it, so it is captured
	// from the readiness probe to keep decisions auditable.
	indexVersion string
	// unmatched remembers the candidates last reported as having no
	// gateway model, so the warning is logged when that set changes
	// rather than on every request.
	unmatched string
}

// NewAutoModelRouter builds the router from configuration.  It returns
// (nil, nil) when no semantic router server is configured, which is the
// signal to the rest of the gateway that automatic routing is off and the
// virtual model must not be published.
func NewAutoModelRouter(cfg *config.Config) (AutoModelRouter, error) {
	if cfg == nil || strings.TrimSpace(cfg.AIGateway.SemanticRouter.ServerURL) == "" {
		return nil, nil
	}

	timeout := time.Duration(cfg.AIGateway.SemanticRouter.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultAutoTimeout
	}
	client, err := semanticrouter.NewClient(cfg.AIGateway.SemanticRouter.ServerURL, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create semantic router client: %w", err)
	}

	modelID := strings.TrimSpace(cfg.AIGateway.SemanticRouter.ModelID)
	if modelID == "" {
		modelID = defaultAutoModelID
	}
	healthTTL := time.Duration(cfg.AIGateway.SemanticRouter.HealthTTLSeconds) * time.Second
	if healthTTL <= 0 {
		healthTTL = defaultAutoHealthTTL
	}

	return &autoModelRouter{
		client:    client,
		modelID:   modelID,
		healthTTL: healthTTL,
	}, nil
}

func (r *autoModelRouter) ModelID() string {
	return r.modelID
}

// Available reports the last known health of the ranking service, and
// refreshes that answer in the background once it has gone stale.  It
// never waits on the network: model listing is on the path of every
// caller, including those that will never request automatic routing, and
// none of them should be delayed by a service they are not going to use.
//
// The answer is false until the first probe lands, so the virtual model
// appears one probe after start-up rather than holding up the first
// listing.  A probe already in flight is not duplicated, so a burst of
// listings still produces a single request.
func (r *autoModelRouter) Available(ctx context.Context) bool {
	r.mu.Lock()
	healthy := r.healthy
	stale := !r.probed || time.Since(r.lastProbeAt) >= r.healthTTL
	if stale && !r.probing {
		r.probing = true
		// The probe outlives the request that triggered it, so it must not
		// be cancelled when that request finishes.
		go r.probe(context.WithoutCancel(ctx), r.generation)
	}
	r.mu.Unlock()
	return healthy
}

// probe refreshes the cached health state.  The client carries its own
// timeout, so this always terminates.
func (r *autoModelRouter) probe(ctx context.Context, generation uint64) {
	started := time.Now()
	readiness, err := r.client.Ready(ctx)
	healthy := err == nil
	probeMS := time.Since(started).Milliseconds()

	indexVersion := ""
	if readiness != nil {
		indexVersion = readiness.IndexVersion
	}

	r.mu.Lock()
	// Always clear the in-flight marker, even when the answer is dropped,
	// or no further probe would ever start.
	r.probing = false
	if r.generation != generation {
		// A failed call decided the service is down while this probe was
		// in flight.  That is both newer and more definitive, so this
		// answer is stale and must not be written back.
		r.mu.Unlock()
		slog.DebugContext(ctx, "discarding a semantic router probe overtaken by a newer result",
			slog.String("model_id", r.modelID),
			slog.Bool("probe_healthy", healthy))
		return
	}
	// Only a change of answer is logged, so a steady service stays quiet
	// while an outage and its recovery are both visible.  Publishing and
	// withdrawing the virtual model is otherwise invisible to operators.
	changed := !r.probed || healthy != r.healthy
	r.healthy = healthy
	r.probed = true
	r.lastProbeAt = time.Now()
	r.generation++
	if healthy {
		r.indexVersion = indexVersion
	}
	r.mu.Unlock()

	if !changed {
		return
	}
	if healthy {
		slog.InfoContext(ctx, "semantic router is reachable, publishing the automatic routing model",
			slog.String("model_id", r.modelID),
			slog.String("index_version", indexVersion),
			slog.Int64("probe_ms", probeMS))
		return
	}
	slog.WarnContext(ctx, "semantic router is unreachable, withdrawing the automatic routing model",
		slog.String("model_id", r.modelID),
		slog.Int64("probe_ms", probeMS),
		slog.Any("error", err))
}

func (r *autoModelRouter) Select(ctx context.Context, input types.AutoRouteInput, candidates []types.Model) (*types.AutoRouteDecision, error) {
	messages, err := toRankMessages(input.Messages)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, newAutoRouteError(autoRouteCodeNotFound,
			"automatic routing has no model to choose from")
	}

	started := time.Now()
	ranked, err := r.client.Rank(ctx, &semanticrouter.RankRequest{
		Messages: messages,
		Tools:    input.Tools,
	})
	elapsedMS := time.Since(started).Milliseconds()
	if err != nil {
		slog.WarnContext(ctx, "semantic router rank call failed",
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("message_count", len(messages)),
			slog.Int("tool_count", len(input.Tools)),
			slog.Int("candidate_count", len(candidates)),
			slog.Any("error", err))
		// A failed call is stronger evidence than a cached readiness probe,
		// so stop advertising the virtual model straight away instead of
		// leaving it listed until the probe expires.
		r.markUnavailable(ctx)
		return nil, newAutoRouteError(autoRouteCodeUnavailable, "semantic router is unavailable: %w", err)
	}

	type catalogueEntry struct {
		modelID   string
		available bool
	}
	byID := make(map[string]catalogueEntry, len(candidates))
	availableCount := 0
	for _, candidate := range candidates {
		key := strings.ToLower(strings.TrimSpace(candidate.ID))
		if key == "" {
			continue
		}
		available := candidate.Availability == nil || candidate.Availability.IsAvailable
		if available {
			availableCount++
		}
		byID[key] = catalogueEntry{modelID: candidate.ID, available: available}
	}
	// A candidate is resolved by its own identifier, or by the upstream
	// model name on a deployment that reports one.  Nothing is inferred
	// from a near-miss: the two sides are expected to use the same model
	// IDs, and a name that does not match is reported rather than guessed
	// at, because the listing holds IDs that a fuzzy rule would confuse
	// (for example deepseek-v4-flash-message next to deepseek-v4-flash).
	lookup := func(candidate semanticrouter.Candidate) (catalogueEntry, bool) {
		for _, name := range []string{candidate.Model, candidate.ProviderModel} {
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" {
				continue
			}
			if entry, ok := byID[key]; ok {
				return entry, true
			}
		}
		return catalogueEntry{}, false
	}

	// An unnamed candidate names no model of this caller's at all, which
	// is a lasting disagreement about names.  An unhealthy one names a
	// real model that is only out of service right now, which resolves on
	// its own — the two must not be reported the same way.
	var unnamed, unhealthy []string
	for _, candidate := range ranked.Ranking {
		entry, ok := lookup(candidate)
		if !ok {
			unnamed = append(unnamed, candidateName(candidate))
			continue
		}
		if !entry.available {
			unhealthy = append(unhealthy, candidateName(candidate))
			continue
		}
		r.reportUnmatched(ctx, unnamed, len(ranked.Ranking))

		indexVersion := ranked.IndexVersion
		if indexVersion == "" {
			indexVersion = r.lastIndexVersion()
		}
		attrs := []any{
			slog.String("model", entry.modelID),
			slog.String("candidate", candidateName(candidate)),
			slog.Int("rank", candidate.Rank),
			// elapsed_ms is the whole round trip as the gateway sees it;
			// service_elapsed_ms is the ranking work the service reports,
			// so the gap between them is network and queueing time.
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("message_count", len(messages)),
			slog.Int("tool_count", len(input.Tools)),
			slog.Int("ranked_count", len(ranked.Ranking)),
			slog.Int("skipped_unnamed", len(unnamed)),
			slog.Int("skipped_unhealthy", len(unhealthy)),
			slog.Int("candidate_count", availableCount),
			slog.String("index_version", indexVersion),
		}
		if candidate.Score > 0 {
			attrs = append(attrs, slog.Float64("score", candidate.Score))
		}
		if ranked.ElapsedMS > 0 {
			attrs = append(attrs, slog.Float64("service_elapsed_ms", ranked.ElapsedMS))
		}
		slog.InfoContext(ctx, "semantic router selected a model", attrs...)

		return &types.AutoRouteDecision{
			ModelID:       entry.modelID,
			BenchmarkID:   candidate.Model,
			Rank:          candidate.Rank,
			Score:         candidate.Score,
			IndexVersion:  indexVersion,
			PolicyVersion: ranked.PolicyVersion,
		}, nil
	}

	// Every candidate that did name one of this caller's models is merely
	// out of service, so the same request succeeds once one recovers.
	// Reporting that as a missing model would tell the caller to stop
	// retrying something that is going to work again.
	if len(unhealthy) > 0 {
		slog.WarnContext(ctx, "every candidate this caller could use is temporarily out of service",
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("ranked_count", len(ranked.Ranking)),
			slog.String("unhealthy_candidates", strings.Join(unhealthy, ",")),
			slog.String("index_version", r.lastIndexVersion()))
		return nil, newAutoRouteError(autoRouteCodeUnavailable,
			"the %d ranked candidates this caller can use are all temporarily unavailable", len(unhealthy))
	}

	// Naming what was offered is what makes this diagnosable: it is almost
	// always a candidate whose name differs from the gateway's model ID.
	slog.WarnContext(ctx, "semantic router returned no candidate this caller can use",
		slog.Int64("elapsed_ms", elapsedMS),
		slog.Int("ranked_count", len(ranked.Ranking)),
		slog.Int("candidate_count", availableCount),
		slog.String("ranked_candidates", strings.Join(rankedNames(ranked.Ranking), ",")),
		slog.String("index_version", r.lastIndexVersion()))
	return nil, newAutoRouteError(autoRouteCodeNotFound,
		"none of the %d ranked candidates names a model available to this caller", len(ranked.Ranking))
}

// markUnavailable records that a real call to the ranking service failed.
// The cached answer is expired rather than refreshed, so the next model
// listing both reports the service as down and starts a new probe — a
// service that comes back is picked up within one listing instead of
// having to wait out the readiness TTL.
func (r *autoModelRouter) markUnavailable(ctx context.Context) {
	r.mu.Lock()
	changed := !r.probed || r.healthy
	r.healthy = false
	r.probed = true
	r.lastProbeAt = time.Time{}
	// Advancing the generation invalidates any probe already in flight, so
	// its older answer cannot resurrect the model behind this decision.
	r.generation++
	r.mu.Unlock()

	if changed {
		slog.WarnContext(ctx, "semantic router call failed, withdrawing the automatic routing model",
			slog.String("model_id", r.modelID))
	}
}

func (r *autoModelRouter) lastIndexVersion() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexVersion
}

// reportUnmatched warns about candidates that were ranked above the chosen
// one but name no gateway model, which is how an operator learns the two
// sides have drifted apart.  Selection still succeeds, so this would be
// invisible otherwise; it is logged only when the set changes, not per
// request.
func (r *autoModelRouter) reportUnmatched(ctx context.Context, skipped []string, ranked int) {
	if len(skipped) == 0 {
		return
	}
	joined := strings.Join(skipped, ",")

	r.mu.Lock()
	changed := r.unmatched != joined
	r.unmatched = joined
	r.mu.Unlock()

	if !changed {
		return
	}
	slog.WarnContext(ctx, "semantic router ranked candidates that name no gateway model, they cannot be selected until the names agree",
		slog.String("candidates", joined),
		slog.Int("skipped_count", len(skipped)),
		slog.Int("ranked_count", ranked))
}

// candidateName is how a candidate is referred to in logs: its own
// identifier, falling back to the upstream model name.
func candidateName(candidate semanticrouter.Candidate) string {
	if name := strings.TrimSpace(candidate.Model); name != "" {
		return name
	}
	return strings.TrimSpace(candidate.ProviderModel)
}

// rankedNames lists what the service offered, as the candidates are named
// in the response.
func rankedNames(ranking []semanticrouter.Candidate) []string {
	names := make([]string, 0, len(ranking))
	for _, candidate := range ranking {
		names = append(names, candidateName(candidate))
	}
	return names
}

// toRankMessages converts the protocol-neutral turn into the ranking
// service's message list, dropping empty turns, trimming an over-long
// history to its most recent messages, and rejecting a turn with no user
// message up front rather than sending a request that cannot be served.
func toRankMessages(messages []types.AutoRouteMessage) ([]semanticrouter.Message, error) {
	converted := make([]semanticrouter.Message, 0, len(messages))
	hasUser := false
	for _, message := range messages {
		if strings.TrimSpace(message.Role) == "" && strings.TrimSpace(message.Content) == "" {
			continue
		}
		if message.Role == "user" {
			hasUser = true
		}
		converted = append(converted, semanticrouter.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	if !hasUser {
		return nil, newAutoRouteError(autoRouteCodeNotFound,
			"automatic routing requires at least one user message in the request")
	}
	if len(converted) > maxAutoRouteMessages {
		converted = converted[len(converted)-maxAutoRouteMessages:]
	}
	return converted, nil
}

// autoModelEntry is the virtual model published in the model list while
// automatic routing is available.  It carries no upstream of its own: the
// Planner replaces it with a real model before any upstream is resolved,
// and AutoRoute marks it so that endpoints which cannot route
// automatically reject it with a clear error instead of trying to proxy
// to nowhere.
func autoModelEntry(modelID string) types.Model {
	return types.Model{
		BaseModel: types.BaseModel{
			ID:      modelID,
			Object:  "model",
			OwnedBy: "OpenCSG",
			Task:    "text-generation",
			// Every candidate the ranking service ranks is a tool-capable
			// chat model, so advertising tool support keeps agent clients
			// from silently dropping their tool definitions.
			SupportFunctionCall: true,
			Metadata:            map[string]any{},
		},
		AutoRoute:    true,
		Availability: &types.ModelAvailability{IsAvailable: true},
	}
}
