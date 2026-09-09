package plan

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// Orchestrator wires the three phases together.  It owns no business logic:
// it calls Extract → Plan → Execute in sequence and delegates error
// rendering to the protocol handler.  All admission decisions (balance,
// quota, safety, routing) live inside the Planner; the Orchestrator never
// inspects plan fields.
type Orchestrator struct {
	planner Planner
}

// NewOrchestrator creates an Orchestrator backed by the given shared Planner.
func NewOrchestrator(planner Planner) *Orchestrator {
	return &Orchestrator{planner: planner}
}

// Dispatch runs the three-phase flow:
//  1. extractor.Extract — protocol-specific metadata extraction
//  2. planner.Plan — protocol-agnostic admission decisions
//  3. handler.Execute — protocol-specific request execution
//
// If the Plan phase rejects the request (returns an error), the Orchestrator
// calls HandlePlanError so the protocol handler can render its own error
// response format.
func (o *Orchestrator) Dispatch(c *gin.Context, extractor MetadataExtractor, handler ProtocolHandler) {
	meta, err := extractor.Extract(c)
	if err != nil {
		return
	}

	p, err := o.planner.Plan(c.Request.Context(), meta)
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
