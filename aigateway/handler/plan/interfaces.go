// Package plan defines the three-phase protocol handler architecture used by
// the AIGateway:
//
//  1. ExtractMetadata (protocol-specific) — parse the request body and
//     extract identity info.
//  2. Plan (protocol-agnostic) — the shared Planner resolves the model target,
//     checks balance/quota/safety, and produces a RequestPlan.  If the request
//     must not proceed the Planner returns an error with an ErrorCode.
//  3. Execute (protocol-specific) — the protocol handler adapts the request,
//     proxies to the upstream, finalizes the response, and records
//     metrics/usage.
//
// The Orchestrator wires these three phases together.  It owns no business
// logic — it simply calls Extract → Plan → Execute in sequence and delegates
// error rendering to the protocol handler.
package plan

import (
	"context"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// PreflightTracer records a preflight span that covers the full request
// lifecycle.  The Orchestrator starts the span in preProcess and stores
// the tracer in the gin context.  Protocol handlers retrieve it via
// GetPreflightTracer to record model-resolution attributes (Execute)
// and errors (HandlePlanError).
type PreflightTracer interface {
	// RecordError records an error on the preflight span and ends it.
	RecordError(err error, errorType string)
	// SetTargetModel records the resolved model target attributes.
	SetTargetModel(requestModel string, target *types.ModelTarget)
	// End ends the preflight span.
	End()
}

// PreflightStarter starts a preflight trace span and returns a
// PreflightTracer.  The handler package provides a concrete adapter
// that wraps its private startPreflightTrace implementation.
type PreflightStarter interface {
	StartTrace(ctx context.Context, api, requestID, userID string) (context.Context, PreflightTracer)
}

// preflightTracerCtxKey is the unified gin.Context key for the
// per-request PreflightTracer.  All protocols share this key because
// each request only hits one protocol.
const preflightTracerCtxKey = "plan.preflight_tracer"

// SetPreflightTracer stores a PreflightTracer in the gin context so
// that Execute and HandlePlanError can access it.  Passing nil clears
// the entry.
func SetPreflightTracer(c *gin.Context, t PreflightTracer) {
	c.Set(preflightTracerCtxKey, t)
}

// GetPreflightTracer retrieves the PreflightTracer from the gin
// context.  Returns nil if none was set.
func GetPreflightTracer(c *gin.Context) PreflightTracer {
	v, exists := c.Get(preflightTracerCtxKey)
	if !exists {
		return nil
	}
	t, _ := v.(PreflightTracer)
	return t
}

// MetadataExtractor is the protocol-specific first phase: it parses the HTTP
// request body and extracts lightweight identity/metadata information.
// On error the extractor MUST write the protocol-specific error response
// to the gin context and return a non-nil error so the Orchestrator
// short-circuits.
type MetadataExtractor interface {
	Extract(c *gin.Context) (*types.RequestMetadata, error)
}

// Planner is the protocol-agnostic second phase.  A single shared Planner
// instance serves all protocols.  It resolves the model target, performs
// routing, checks balance/quota/content-safety, and returns a RequestPlan.
//
// If the Planner decides the request must not proceed it returns a non-nil
// error.  The returned RequestPlan is always non-nil (even on error) so the
// protocol handler can read ErrorCode and any partially populated fields
// (e.g. ModelTarget may be set even if a later check fails) to attribute
// the error to the right model.
type Planner interface {
	Plan(c *gin.Context, meta *types.RequestMetadata) (*types.RequestPlan, error)
}

// MetricsEnricher enriches the RequestMetrics with model target info
// (model name, provider, upstream ID, stream flag) after the Plan phase
// resolves the model target.  The Orchestrator calls it once after Plan
// returns, covering both the success and error paths so every protocol
// handler no longer needs to call SetMetricsModelTarget individually.
type MetricsEnricher interface {
	SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool)
}

// ProtocolHandler is the protocol-specific third phase.  It receives the
// RequestPlan and is responsible for:
//   - Adapting the request for the upstream protocol (request body transform
//     + response writer creation).
//   - Executing the reverse proxy.
//   - Finalizing the response and recording usage/metrics.
//   - Rendering protocol-specific error responses when the Plan phase
//     rejects the request.
type ProtocolHandler interface {
	// Execute adapts and proxies the request.  This method owns the full
	// execution lifecycle: Adapt (request transform + writer creation),
	// proxy, finalize, usage/metrics recording.
	//
	// The returned error is for logging only — by the time Execute returns
	// the HTTP response may already be committed (e.g. after streaming
	// begins), so the Orchestrator does not render an error response on
	// Execute failure.
	Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error

	// HandlePlanError renders the protocol-specific error response for a
	// Plan-phase rejection.  The RequestPlan's ErrorCode identifies the
	// category (model not found, insufficient balance, quota exceeded,
	// protocol disabled, sensitive content, etc.).
	HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error)
}

// OrchestratorInterface wires the three phases together.
type OrchestratorInterface interface {
	Dispatch(c *gin.Context, extractor MetadataExtractor, handler ProtocolHandler)
}
