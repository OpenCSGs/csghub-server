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
	metricsEnricher   MetricsEnricher
}

// NewPlanner constructs a Planner from its four dependency interfaces.
// The handler package provides concrete adapters at the composition root.
func NewPlanner(mr ModelResolver, bc BalanceChecker, ulc UsageLimitChecker, cs ContentSafetyChecker, me MetricsEnricher) Planner {
	return &plannerImpl{
		modelResolver:     mr,
		balanceChecker:    bc,
		usageLimitChecker: ulc,
		contentSafety:     cs,
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

	// 2. Protocol Routing.
	decision, err := protocol.ResolveRouting(types.Protocol(meta.Protocol), protocol.RoutingTarget{
		ModelID:          mt.Model.ID,
		Target:           mt.Target,
		CSGHubHosted:     mt.Model.SvcName != "",
		RuntimeFramework: mt.Model.RuntimeFramework,
		ImageID:          mt.Model.ImageID,
		ProtocolOverride: mt.Upstream.MetadataProtocol(),
	})
	if err != nil {
		pl.ErrorCode = types.PlanErrUnknown
		return pl, err
	}
	pl.RouteMode = string(decision.Mode)
	pl.AdapterKind = string(decision.AdapterKind)
	pl.UpstreamProtocol = string(decision.UpstreamProtocol)
	if decision.Mode == protocol.ModeAdapter {
		pl.UpstreamCap = types.CapabilityFor(decision.UpstreamProtocol)
	}

	// 3. Disabled — return error so the Orchestrator routes to HandlePlanError.
	// No need to check balance/quota/safety for a rejected request.
	if decision.Mode == protocol.ModeDisabled {
		pl.ErrorCode = types.PlanErrDisabled
		pl.BackendURL = mt.Target
		return pl, fmt.Errorf("protocol %s is not available for this model", meta.Protocol)
	}

	// 4. Balance check (respects SkipBalance).
	if !mt.Model.SkipBalance() {
		if err := p.balanceChecker.CheckBalance(ctx, meta.TenantID); err != nil {
			pl.ErrorCode = categorizePlanError(err)
			return pl, err
		}
	}
	pl.BalanceOK = true

	// 5. Resolve backend URL.
	pl.BackendURL = decision.BackendURL
	if pl.BackendURL == "" {
		pl.BackendURL = mt.Target
	}

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

	// 7. Content-safety check (input).
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
