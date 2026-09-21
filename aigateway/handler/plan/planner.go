package plan

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/errorx"
)

// plannerImpl is the protocol-agnostic request planner.  It is shared across
// all inference protocols in the aigateway module: every request goes through
// the same model resolution, routing, balance, quota, and content-safety
// checks.
//
// If the Planner decides a request must not proceed (model not found,
// insufficient balance, quota exceeded, protocol disabled, sensitive content),
// it returns a non-nil error together with a partially populated RequestPlan
// whose ErrorCode identifies the category.  The Orchestrator passes this to
// the protocol handler's HandlePlanError so it can render the correct
// protocol-specific error response.
type plannerImpl struct {
	modelResolver     ModelResolver
	balanceChecker    BalanceChecker
	usageLimitChecker UsageLimitChecker
	contentSafety     ContentSafetyChecker
	admissionChecker  AdmissionChecker
	metricsEnricher   MetricsEnricher
}

// NewPlanner constructs a Planner from its dependency interfaces.
// The handler package provides concrete adapters at the composition root.
// admissionChecker may be nil in tests; admission is then skipped.
func NewPlanner(mr ModelResolver, bc BalanceChecker, ulc UsageLimitChecker, cs ContentSafetyChecker, ac AdmissionChecker, me MetricsEnricher) Planner {
	return &plannerImpl{
		modelResolver:     mr,
		balanceChecker:    bc,
		usageLimitChecker: ulc,
		contentSafety:     cs,
		admissionChecker:  ac,
		metricsEnricher:   me,
	}
}

// Plan produces a RequestPlan from the RequestMetadata.
func (p *plannerImpl) Plan(c *gin.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	ctx := c.Request.Context()
	pl := &types.RequestPlan{}

	// 1. Model Resolution.
	mt, err := p.modelResolver.ResolveModelTarget(ctx, meta.TenantID, meta.Model, meta.Headers, ResolveOptions{
		RequiredUpstreamID: meta.RequiredUpstreamID,
	})
	// Enrich metrics with the resolved model target right after resolution,
	// before error checking.  When resolution fails (mt == nil) the enricher
	// falls back to the requested model ID so the error is still attributed.
	if p.metricsEnricher != nil {
		p.metricsEnricher.SetModelTarget(c, meta.Model, mt, meta.Streaming)
	}
	if err != nil {
		pl.ErrorCode = categorizePlanError(err)
		return pl, err
	}
	if mt == nil {
		pl.ErrorCode = types.PlanErrModelNotFound
		return pl, fmt.Errorf("model '%s' not found", meta.Model)
	}
	pl.ModelTarget = mt

	// 2. Protocol Routing. The helper also resolves the backend URL and
	// rejects disabled protocol/upstream combinations (steps 3 and 5 of the
	// original inline flow).
	if err := applyProtocolRouting(pl, meta, mt); err != nil {
		if pl.ErrorCode == types.PlanErrDisabled {
			return pl, err
		}
		pl.ErrorCode = types.PlanErrUnknown
		return pl, err
	}

	// 4. Balance check (respects SkipBalance).
	if !mt.Model.SkipBalance() {
		if err := p.balanceChecker.CheckBalance(ctx, meta.TenantID); err != nil {
			pl.ErrorCode = categorizePlanError(err)
			return pl, err
		}
	}
	pl.BalanceOK = true

	// 6. Usage-limit check — only for token-generating protocols.
	// Non-token endpoints (image, video, audio, ocr, rerank, embedding,
	// speech) have non-token billing models and do not participate in the
	// token-window rate limiter.
	if shouldCheckUsageLimit(meta.Task) {
		if err := p.usageLimitChecker.CheckUsageLimit(ctx, meta.TenantID, mt.Model, pl.BackendURL); err != nil {
			pl.ErrorCode = categorizePlanError(err)
			return pl, err
		}
	}
	pl.UsageLimitOK = true

	// 7. Capacity admission control — the first backpressure layer in front
	// of the runtimes. It runs BEFORE the content-safety check for two
	// reasons:
	//   - The sensitive check is the expensive step (whitelist query +
	//     moderation RPC): requests that have no capacity to run should not
	//     pay for it ("no资格执行就别审").
	//   - A capacity re-selection MUST happen before the sensitive check:
	//     the check's whitelist targets are built from the upstream
	//     provider, so it must observe the FINAL upstream, not the
	//     router-picked one that admission may replace.
	// A sensitive rejection after acquire releases the just-acquired lease
	// immediately (Orchestrator safety net / error return paths), so a
	// policy-blocked request never queues on upstream capacity. The RPM
	// counter keeps its one immutable increment — the attempt genuinely
	// reached the upstream's admission gate.
	if p.admissionChecker != nil && shouldAdmitCapacity(meta.Task) {
		outcome, admissionErr := p.admissionChecker.CheckAdmission(ctx, meta, mt)
		if admissionErr != nil {
			// Checker programming errors surface as internal errors; the
			// checker itself never fails on Redis issues (fail-open).
			pl.ErrorCode = types.PlanErrInternal
			return pl, admissionErr
		}
		if outcome != nil && outcome.Decision != nil {
			decision := outcome.Decision
			// Record the decision on the plan for both observability and
			// the Orchestrator's lease-release safety net.
			pl.Admission = decision
			if decision.Action != types.AdmissionAdmit {
				pl.ErrorCode = types.PlanErrCapacityExceeded
				return pl, &types.AdmissionDeniedError{Decision: decision}
			}
			// Capacity-aware fallback: the admission layer re-selected a
			// different upstream of the same candidate set. Re-apply the
			// protocol routing for the new target and update the plan —
			// BEFORE the sensitive check below, so the safety gate sees the
			// final upstream (its provider feeds the whitelist targets).
			if outcome.ReSelectedTarget != nil {
				if err := applyProtocolRouting(pl, meta, outcome.ReSelectedTarget); err != nil {
					pl.ErrorCode = types.PlanErrModelUnavailable
					return pl, err
				}
				pl.ModelTarget = outcome.ReSelectedTarget
				pl.BackendURL = outcome.ReSelectedTarget.Target
				// The sensitive check must run against the re-selected
				// target; mt is the local the rest of the plan uses.
				mt = outcome.ReSelectedTarget
			}
		}
	}

	// 8. Content-safety check (input) — runs against the FINAL upstream
	// (possibly admission-re-selected above; its provider feeds the
	// whitelist targets), and only for requests that passed admission, so
	// the expensive moderation RPC is never paid for requests that had no
	// capacity to run anyway.
	promptText := meta.PromptText()
	if promptText != "" {
		isSensitive, message, checkErr := p.contentSafety.Check(
			ctx, mt.Model, promptText, meta.TenantID, meta.Task, meta.Streaming, mt.Upstream.Provider,
		)
		if checkErr != nil {
			slog.WarnContext(ctx, "planner sensitive check error", slog.Any("error", checkErr))
		} else if isSensitive {
			pl.Safety = &types.SafetyDecision{IsSensitive: true, Message: message}
			pl.ErrorCode = types.PlanErrSensitive
			return pl, fmt.Errorf("content blocked due to safety policy")
		}
	}

	return pl, nil
}

// applyProtocolRouting resolves the client protocol against the upstream,
// stores the routing decision and the backend URL on the plan, and rejects
// disabled protocol/upstream combinations. It runs once for the
// router-picked target and again when capacity admission re-selects a
// different upstream, whose protocol may differ.
func applyProtocolRouting(pl *types.RequestPlan, meta *types.RequestMetadata, mt *types.ModelTarget) error {
	decision, err := protocol.ResolveRouting(types.Protocol(meta.Protocol), protocol.RoutingTarget{
		ModelID:          mt.Model.ID,
		Target:           mt.Target,
		CSGHubHosted:     mt.Model.SvcName != "",
		RuntimeFramework: mt.Model.RuntimeFramework,
		ImageID:          mt.Model.ImageID,
		ProtocolOverride: mt.Upstream.MetadataProtocol(),
	})
	if err != nil {
		return err
	}
	pl.RouteMode = string(decision.Mode)
	pl.AdapterKind = string(decision.AdapterKind)
	pl.UpstreamProtocol = string(decision.UpstreamProtocol)
	if decision.Mode == protocol.ModeAdapter {
		pl.UpstreamCap = types.CapabilityFor(decision.UpstreamProtocol)
	}
	pl.BackendURL = decision.BackendURL
	if pl.BackendURL == "" {
		pl.BackendURL = mt.Target
	}
	if decision.Mode == protocol.ModeDisabled {
		// Return error so the Orchestrator routes to HandlePlanError. No
		// need to check balance/quota/safety/admission for a rejected
		// request.
		pl.BackendURL = mt.Target
		pl.ErrorCode = types.PlanErrDisabled
		return fmt.Errorf("protocol %s is not available for this model", meta.Protocol)
	}
	return nil
}

// Ensure plannerImpl satisfies the Planner interface.
var _ Planner = (*plannerImpl)(nil)

// categorizePlanError inspects a Plan-phase error and returns its category.
// Protocol handlers use this to select the correct HTTP status and error type.
func categorizePlanError(err error) types.PlanErrorCategory {
	if err == nil {
		return types.PlanErrUnknown
	}

	// Check for CodedError (model not found, not running, unavailable, etc.).
	var ce CodedError
	if errors.As(err, &ce) {
		switch ce.ModelErrorCode() {
		case "model_not_found", "model_not_running":
			return types.PlanErrModelNotFound
		case "model_unavailable", "required_upstream_unavailable":
			return types.PlanErrModelUnavailable
		case "internal_error", "cluster_not_found":
			return types.PlanErrInternal
		}
	}

	// Check for insufficient balance.
	if errors.Is(err, errorx.ErrInsufficientBalance) {
		return types.PlanErrInsufficientBalance
	}

	// Check for usage limit exceeded.
	if component.IsUsageLimitExceeded(err) {
		return types.PlanErrUsageLimitExceeded
	}

	return types.PlanErrUnknown
}

// shouldCheckUsageLimit reports whether the task participates in the
// token-window rate limiter (CheckUsageLimit / CommitUsageLimitFromUsage).
// Only token-generating protocols (chat, responses, messages) are gated;
// non-token endpoints (image, video, audio, ocr, rerank, embedding, speech)
// consume tokens but use different billing models and are excluded from the
// token-window limiter.
func shouldCheckUsageLimit(task string) bool {
	switch task {
	case "chat", "responses", "messages":
		return true
	default:
		return false
	}
}

// shouldAdmitCapacity reports whether the task participates in capacity
// admission. Token-generating protocols (chat, responses, messages) are
// gated by all CapacityPolicy dimensions; image generation is gated too but
// always without a TPM reservation (its parsed body reports multimodal
// content, so the checker passes EstimatedTokens <= 0 and only concurrency
// and RPM bind). Other modal endpoints (video, audio, ocr, rerank,
// embedding, speech) bypass admission in v1.
func shouldAdmitCapacity(task string) bool {
	switch task {
	case "chat", "responses", "messages", "text-to-image":
		return true
	default:
		return false
	}
}
