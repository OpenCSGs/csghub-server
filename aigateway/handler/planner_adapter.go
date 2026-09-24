package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/types"
)

// modelResolverAdapter wraps OpenAIHandlerImpl.resolveModelTarget to satisfy
// plan.ModelResolver.  It converts the handler-private *resolvedModelTarget
// to the public *types.ModelTarget so the Planner never depends on handler
// internals.
type modelResolverAdapter struct {
	handler *OpenAIHandlerImpl
}

func (a *modelResolverAdapter) ResolveModelTarget(ctx context.Context, nsUUID, modelID string, headers http.Header, opts plan.ResolveOptions) (*types.ModelTarget, error) {
	resolved, err := a.handler.resolveModelTargetWithOptions(ctx, nsUUID, modelID, headers, modelTargetResolveOptions{
		RequiredUpstreamID: opts.RequiredUpstreamID,
	})
	if err != nil {
		return nil, err
	}
	return toTypesModelTarget(resolved), nil
}

// contentSafetyAdapter wraps the existing sensitive policy (which exposes the
// Responses-named method CheckResponsesSensitive) to satisfy
// plan.ContentSafetyChecker.  It returns a simplified (bool, string, error)
// contract so the Planner never depends on *rpc.CheckResult.
type contentSafetyAdapter struct {
	policy component.SensitivePolicy
}

func (a *contentSafetyAdapter) Check(ctx context.Context, model *types.Model, promptText, nsUUID, task string, streaming bool, provider string) (bool, string, error) {
	// Only token-generating protocols (chat/responses/messages) go through
	// the SensitivePolicy gate here.  Other tasks (text-to-image,
	// text-to-video, audio, ocr, embedding, rerank) either skip content
	// safety entirely or use CheckImagePrompts directly in their Execute
	// phase, which bypasses the SensitivePolicy gate intentionally.
	switch task {
	case "chat", "responses", "messages":
		// fall through
	default:
		return false, "", nil
	}

	shouldCheck, result, err := a.policy.CheckResponsesSensitive(ctx, model, promptText, nsUUID, streaming, provider)
	if err != nil {
		return false, "", err
	}
	if result == nil {
		return false, "", nil
	}
	return shouldCheck && result.IsSensitive, result.Reason, nil
}

// newPlannerDeps builds the dependency set needed by plan.NewPlanner from
// an OpenAIHandlerImpl.  The openai component satisfies several of the
// interfaces directly, including automatic model selection.
func newPlannerDeps(h *OpenAIHandlerImpl) plan.PlannerDeps {
	return plan.PlannerDeps{
		ModelResolver:     &modelResolverAdapter{handler: h},
		BalanceChecker:    h.openaiComponent,
		UsageLimitChecker: h.openaiComponent,
		ContentSafety:     &contentSafetyAdapter{policy: h.sensitivePolicy},
		AdmissionChecker:  newAdmissionChecker(h),
		MetricsEnricher:   metricsEnricherAdapter{},
		AutoModelSelector: h.openaiComponent,
	}
}

// metricsEnricherAdapter implements plan.MetricsEnricher by delegating to the
// build-tag-gated SetMetricsModelTarget helper.  In CE builds the helper is a
// no-op, so this adapter is also a no-op.
type metricsEnricherAdapter struct{}

func (metricsEnricherAdapter) SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool) {
	var rt *resolvedModelTarget
	if target != nil {
		rt = modelTargetToResolved(target)
	}
	SetMetricsModelTarget(SetMetricsModelParams{
		C:           c,
		ModelID:     modelID,
		ModelTarget: rt,
		IsStream:    isStream,
	})
}

// newOrchestrator builds a fully-wired Orchestrator from an OpenAIHandlerImpl.
// It encapsulates planner dependency wiring (including metrics enrichment and
// capacity admission) and preflight tracing so protocol handlers don't repeat
// boilerplate.
func newOrchestrator(h *OpenAIHandlerImpl) *plan.Orchestrator {
	planner := plan.NewPlanner(newPlannerDeps(h), plan.WithQueueMaxRePlans(h.queueMaxRePlans()))
	if releaser := newAdmissionReleaser(h); releaser != nil {
		return plan.NewOrchestrator(planner, &preflightStarterAdapter{}, releaser)
	}
	// CE: no admission, no lease-release safety net needed.
	return plan.NewOrchestrator(planner, &preflightStarterAdapter{})
}

// queueMaxRePlans returns the configured bound for admission-queue reroute
// re-plans.
func (h *OpenAIHandlerImpl) queueMaxRePlans() int {
	if h.config == nil {
		return 1
	}
	return h.config.AIGateway.CapacityAdmission.QueueMaxRePlans
}

// toTypesModelTarget converts the handler-package-private resolvedModelTarget
// to the types-package ModelTarget.
func toTypesModelTarget(r *resolvedModelTarget) *types.ModelTarget {
	if r == nil {
		return nil
	}
	return &types.ModelTarget{
		Model:          r.Model,
		Upstream:       r.Upstream,
		Target:         r.Target,
		Host:           r.Host,
		ModelName:      r.ModelName,
		AttemptTargets: r.AttemptTargets,
	}
}
