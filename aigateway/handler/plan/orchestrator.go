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
// never inspects plan fields — with one deliberate exception: the capacity
// admission lease lifecycle. When the Planner acquired a Redis lease, the
// Orchestrator guarantees it is finalized (released) after Execute returns,
// on every path including errors and panics.
type Orchestrator struct {
	planner           Planner
	preflightStarter  PreflightStarter // nil = preflight tracing disabled (test scenarios)
	admissionReleaser AdmissionReleaser
}

// NewOrchestrator creates an Orchestrator backed by the given shared Planner.
// The preflightStarter starts the preflight trace span; pass nil to disable
// preflight tracing (useful in unit tests). The optional admissionReleaser
// enables the admission lease safety net; pass nil to skip it.
func NewOrchestrator(planner Planner, preflightStarter PreflightStarter, admissionReleaser ...AdmissionReleaser) *Orchestrator {
	o := &Orchestrator{planner: planner, preflightStarter: preflightStarter}
	if len(admissionReleaser) > 0 {
		o.admissionReleaser = admissionReleaser[0]
	}
	return o
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
	if o.admissionReleaser != nil && p != nil {
		// Capacity admission lease safety net. The lease is read from the
		// plan at DEFER EXECUTION time, not registration time: a lease
		// acquired later (e.g. the availability fallback re-acquiring a
		// pinned lease in the attempt loop after a fail-open Plan) must be
		// released too, otherwise the renewer would keep it alive forever.
		// Execute blocks until the response is fully written, so this defer
		// runs after the upstream attempt finished — covering success,
		// execute errors and panics. The release is ownership-guarded and
		// idempotent: when the protocol's async usage commit already
		// finalized the lease (with real usage), this is a Redis no-op.
		defer o.admissionReleaser.ReleaseAdmission(c, p)
	}
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
