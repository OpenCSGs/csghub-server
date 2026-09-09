package handler

import (
	"context"
	"net/http"

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

func (a *modelResolverAdapter) ResolveModelTarget(ctx context.Context, username, modelID string, headers http.Header) (*types.ModelTarget, error) {
	resolved, err := a.handler.resolveModelTarget(ctx, username, modelID, headers)
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

func (a *contentSafetyAdapter) Check(ctx context.Context, model *types.Model, promptText, tenantID string, streaming bool, provider string) (bool, string, error) {
	shouldCheck, result, err := a.policy.CheckResponsesSensitive(ctx, model, promptText, tenantID, streaming, provider)
	if err != nil {
		return false, "", err
	}
	if result == nil {
		return false, "", nil
	}
	return shouldCheck && result.IsSensitive, result.Reason, nil
}

// newPlannerDeps builds the four dependency interfaces needed by
// plan.NewPlanner from an OpenAIHandlerImpl.
func newPlannerDeps(h *OpenAIHandlerImpl) (plan.ModelResolver, plan.BalanceChecker, plan.UsageLimitChecker, plan.ContentSafetyChecker) {
	return &modelResolverAdapter{handler: h},
		h.openaiComponent,
		h.openaiComponent,
		&contentSafetyAdapter{policy: h.sensitivePolicy}
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
