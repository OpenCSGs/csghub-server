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

	defaultAutoModelID = "auto"
	// maxAutoRouteModelAttempts caps how many ranked models one turn may
	// be tried against before the request is answered with the last
	// failure.  Automatic routing hides which model serves a turn, so a
	// model that fails is worth routing around rather than reporting; a
	// bounded number of attempts keeps that from turning one slow client
	// request into an unbounded walk down the ranking.
	maxAutoRouteModelAttempts = 3
	defaultAutoTimeout        = 5 * time.Second
	defaultAutoHealthTTL      = 5 * time.Minute
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
	rankReq, err := toRankRequest(input)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, newAutoRouteError(autoRouteCodeNotFound,
			"automatic routing has no model to choose from")
	}

	started := time.Now()
	ranked, err := r.client.Rank(ctx, rankReq)
	elapsedMS := time.Since(started).Milliseconds()
	if err != nil {
		slog.WarnContext(ctx, "semantic router rank call failed",
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("history_count", len(rankReq.History)),
			slog.Int("current_input_count", len(rankReq.CurrentInput)),
			slog.Int("tool_count", len(rankReq.Tools)),
			slog.Int("candidate_count", len(candidates)),
			slog.Any("error", err))
		// A failed call is stronger evidence than a cached readiness probe,
		// so stop advertising the virtual model straight away instead of
		// leaving it listed until the probe expires.
		r.markUnavailable(ctx)
		return nil, newAutoRouteError(autoRouteCodeUnavailable, "semantic router is unavailable: %w", err)
	}

	matched := matchRankedCandidates(ranked.ModelList, candidates, maxAutoRouteModelAttempts)
	usable, unnamed, unhealthy, availableCount := matched.usable, matched.unnamed, matched.unhealthy, matched.availableCount

	if len(usable) > 0 {
		chosen := usable[0]
		r.reportUnmatched(ctx, unnamed, len(ranked.ModelList))

		indexVersion := r.lastIndexVersion()
		attrs := []any{
			slog.String("model", chosen.ModelID),
			slog.String("candidate", chosen.BenchmarkID),
			slog.Int("rank", chosen.Rank),
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("history_count", len(rankReq.History)),
			slog.Int("current_input_count", len(rankReq.CurrentInput)),
			slog.Int("tool_count", len(rankReq.Tools)),
			slog.Int("ranked_count", len(ranked.ModelList)),
			slog.Int("skipped_unnamed", len(unnamed)),
			slog.Int("skipped_unhealthy", len(unhealthy)),
			slog.Int("candidate_count", availableCount),
			slog.String("index_version", indexVersion),
		}
		alternates := usable[1:]
		if len(alternates) > 0 {
			attrs = append(attrs, slog.String("alternates", strings.Join(candidateModelIDs(alternates), ",")))
		}
		slog.InfoContext(ctx, "semantic router selected a model", attrs...)

		return &types.AutoRouteDecision{
			ModelID:      chosen.ModelID,
			BenchmarkID:  chosen.BenchmarkID,
			Rank:         chosen.Rank,
			IndexVersion: indexVersion,
			SessionID:    ranked.SessionID,
			TurnID:       ranked.TurnID,
			Alternates:   alternates,
		}, nil
	}

	// Every candidate that did name one of this caller's models is merely
	// out of service, so the same request succeeds once one recovers.
	// Reporting that as a missing model would tell the caller to stop
	// retrying something that is going to work again.
	if len(unhealthy) > 0 {
		slog.WarnContext(ctx, "every candidate this caller could use is temporarily out of service",
			slog.Int64("elapsed_ms", elapsedMS),
			slog.Int("ranked_count", len(ranked.ModelList)),
			slog.String("unhealthy_candidates", strings.Join(unhealthy, ",")),
			slog.String("index_version", r.lastIndexVersion()))
		return nil, newAutoRouteError(autoRouteCodeUnavailable,
			"the %d ranked candidates this caller can use are all temporarily unavailable", len(unhealthy))
	}

	// Naming what was offered is what makes this diagnosable: it is almost
	// always a candidate whose name differs from the gateway's model ID.
	slog.WarnContext(ctx, "semantic router returned no candidate this caller can use",
		slog.Int64("elapsed_ms", elapsedMS),
		slog.Int("ranked_count", len(ranked.ModelList)),
		slog.Int("candidate_count", availableCount),
		slog.String("ranked_candidates", strings.Join(ranked.ModelList, ",")),
		slog.String("index_version", r.lastIndexVersion()))
	return nil, newAutoRouteError(autoRouteCodeNotFound,
		"none of the %d ranked candidates names a model available to this caller", len(ranked.ModelList))
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

// toSemanticMessages converts protocol-neutral messages to the ranking
// service's message type.  The messages are already flattened text here:
// the split in the types package dropped empty turns.
func toSemanticMessages(messages []types.AutoRouteMessage) []semanticrouter.Message {
	converted := make([]semanticrouter.Message, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, semanticrouter.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return converted
}

// toRankRequest turns the protocol-neutral turn into the ranking service's
// request, trimming an over-long history to its most recent messages and
// rejecting a turn with no current user message up front rather than
// sending a request that cannot be served.
func toRankRequest(input types.AutoRouteInput) (*semanticrouter.RankRequest, error) {
	history := input.History
	if len(history) > maxAutoRouteMessages {
		history = history[len(history)-maxAutoRouteMessages:]
	}
	hasUser := false
	for _, message := range input.CurrentInput {
		if message.Role == "user" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		return nil, newAutoRouteError(autoRouteCodeNotFound,
			"automatic routing requires at least one user message in the request")
	}
	return &semanticrouter.RankRequest{
		SystemPrompt: input.SystemPrompt,
		Tools:        input.Tools,
		History:      toSemanticMessages(history),
		CurrentInput: toSemanticMessages(input.CurrentInput),
		SessionID:    input.SessionID,
		TurnID:       input.TurnID,
	}, nil
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

// candidateModelIDs names the models held in reserve for one turn, so the
// selection log shows what the request can still fall back to.
func candidateModelIDs(candidates []types.AutoRouteCandidate) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ModelID)
	}
	return ids
}

// rankedCandidateMatch is the outcome of reconciling one ranking against
// the models a caller can actually use.  Usable holds the models to serve
// the turn with, in rank order and no more than the limit; the two skip
// lists are kept apart because they mean different things and produce
// different errors — an unnamed candidate names no model of this caller's
// at all, which is a lasting disagreement about names, while an unhealthy
// one names a real model that is only out of service right now.
type rankedCandidateMatch struct {
	usable         []types.AutoRouteCandidate
	unnamed        []string
	unhealthy      []string
	availableCount int
}

// matchRankedCandidates reconciles the ranked model names with the caller's
// own models.  It is a pure function of its inputs so the matching rules —
// which name resolves to which model, what counts as usable, and how many
// models one turn may hold — can be verified without a ranking service, a
// model catalogue or a router.
//
// The service reports gateway-callable model names directly, so a candidate
// is resolved by exact, case-insensitive name.  Nothing is inferred from a
// near-miss: the two sides are expected to use the same model IDs, and a
// name that does not match is reported rather than guessed at, because the
// listing holds IDs that a fuzzy rule would confuse (for example
// deepseek-v4-flash-message next to deepseek-v4-flash).
//
// Ranking is walked only until limit models are in hand, so a long ranking
// is not walked to the end for candidates that will never be tried.
func matchRankedCandidates(ranked []string, candidates []types.Model, limit int) rankedCandidateMatch {
	type catalogueEntry struct {
		modelID   string
		available bool
	}
	byID := make(map[string]catalogueEntry, len(candidates))
	match := rankedCandidateMatch{usable: make([]types.AutoRouteCandidate, 0, limit)}
	for _, candidate := range candidates {
		key := strings.ToLower(strings.TrimSpace(candidate.ID))
		if key == "" {
			continue
		}
		available := candidate.Availability == nil || candidate.Availability.IsAvailable
		if available {
			match.availableCount++
		}
		byID[key] = catalogueEntry{modelID: candidate.ID, available: available}
	}

	for i, name := range ranked {
		key := strings.ToLower(strings.TrimSpace(name))
		entry, ok := byID[key]
		if !ok {
			match.unnamed = append(match.unnamed, name)
			continue
		}
		if !entry.available {
			match.unhealthy = append(match.unhealthy, name)
			continue
		}
		match.usable = append(match.usable, types.AutoRouteCandidate{
			ModelID:     entry.modelID,
			BenchmarkID: name,
			Rank:        i + 1,
		})
		if len(match.usable) >= limit {
			break
		}
	}
	return match
}
