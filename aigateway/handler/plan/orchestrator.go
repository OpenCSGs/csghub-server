package plan

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// Orchestrator wires the three phases together.  It owns no business logic:
// it calls preProcess → Extract → Plan → Execute in sequence and delegates
// error rendering to the protocol handler.  All admission decisions
// (balance, quota, safety, routing) live inside the Planner; the Orchestrator
// never inspects plan fields.
type Orchestrator struct {
	planner          Planner
	preflightStarter PreflightStarter // nil = preflight tracing disabled (test scenarios)
}

// NewOrchestrator creates an Orchestrator backed by the given shared Planner.
// The preflightStarter starts the preflight trace span; pass nil to disable
// preflight tracing (useful in unit tests).
func NewOrchestrator(planner Planner, preflightStarter PreflightStarter) *Orchestrator {
	return &Orchestrator{planner: planner, preflightStarter: preflightStarter}
}

// preProcess runs before Extract.  It starts the preflight trace span and
// stores the tracer in the gin context so Execute/HandlePlanError can record
// model-resolution attributes and errors on the same span.
//
// Future Extract-preprocessing logic should be added here, keeping Dispatch
// focused on the three-phase flow.
func (o *Orchestrator) preProcess(c *gin.Context) {
	if o.preflightStarter == nil {
		return
	}
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	requestID := commontrace.GetTraceIDInGinContext(c)
	ctx, tracer := o.preflightStarter.StartTrace(c.Request.Context(), c.FullPath(), requestID, nsUUID)
	c.Request = c.Request.WithContext(ctx)
	SetPreflightTracer(c, tracer)
}

// Dispatch runs the three-phase flow:
//  0. preProcess — start preflight trace (if configured)
//  1. extractor.Extract — protocol-specific metadata extraction
//  2. planner.Plan — protocol-agnostic admission decisions
//  3. handler.Execute — protocol-specific request execution
//
// If the Plan phase rejects the request (returns an error), the Orchestrator
// calls HandlePlanError so the protocol handler can render its own error
// response format.
func (o *Orchestrator) Dispatch(c *gin.Context, extractor MetadataExtractor, handler ProtocolHandler) {
	o.preProcess(c)
	if pt := GetPreflightTracer(c); pt != nil {
		defer pt.End()
	}

	meta, err := extractor.Extract(c)
	if err != nil {
		// The extractor has already rendered the protocol-specific error
		// response.  Record the error on the preflight span so it is not
		// lost from traces (RecordError internally ends the span; the
		// deferred End() is idempotent).
		if pt := GetPreflightTracer(c); pt != nil {
			pt.RecordError(err, "extract_error")
		}
		return
	}

	p, err := o.planner.Plan(c, meta)
	if err != nil {
		handler.HandlePlanError(c, meta, p, err)
		return
	}

	if err := handler.Execute(c, meta, p); err != nil {
		slog.WarnContext(c.Request.Context(), "protocol handler execute error",
			slog.String("protocol", meta.Protocol),
			slog.Any("error", err))
	}
}

// Ensure Orchestrator satisfies the OrchestratorInterface.
var _ OrchestratorInterface = (*Orchestrator)(nil)
