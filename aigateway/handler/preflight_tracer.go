package handler

import (
	"context"

	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/types"
)

// preflightStarterAdapter implements plan.PreflightStarter by wrapping the
// handler-private startPreflightTrace.  It is passed to the Orchestrator at
// construction time so the Orchestrator can start preflight spans without
// depending on the handler package.
type preflightStarterAdapter struct{}

func (a *preflightStarterAdapter) StartTrace(ctx context.Context, api, requestID, userID string) (context.Context, plan.PreflightTracer) {
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       api,
		RequestID: requestID,
		UserID:    userID,
	})
	return ctx, &preflightTracerAdapter{trace: preflight}
}

// preflightTracerAdapter wraps *preflightTrace to satisfy plan.PreflightTracer.
// It converts *types.ModelTarget to *resolvedModelTarget for SetTargetModel.
type preflightTracerAdapter struct {
	trace *preflightTrace
}

func (a *preflightTracerAdapter) RecordError(err error, errorType string) {
	a.trace.RecordError(err, errorType)
}

func (a *preflightTracerAdapter) SetTargetModel(requestModel string, target *types.ModelTarget) {
	a.trace.SetTargetModel(requestModel, modelTargetToResolved(target))
}

func (a *preflightTracerAdapter) End() {
	a.trace.End()
}

// modelTargetToResolved converts *types.ModelTarget to the handler-private
// *resolvedModelTarget.  Used by preflight tracer adapters and metrics.
func modelTargetToResolved(target *types.ModelTarget) *resolvedModelTarget {
	if target == nil {
		return nil
	}
	return &resolvedModelTarget{
		Model:          target.Model,
		Upstream:       target.Upstream,
		Target:         target.Target,
		Host:           target.Host,
		ModelName:      target.ModelName,
		AttemptTargets: target.AttemptTargets,
	}
}
