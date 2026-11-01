package types

import (
	"net/http"

	commonType "opencsg.com/csghub-server/common/types"
)

// RequestMetadata describes "what this request is" via lightweight parsing.
// It is produced by the ExtractMetadata phase and does not involve any
// business logic (no DB / Redis queries).
type RequestMetadata struct {
	// Protocol identifies the client protocol: "chat", "messages", "responses", ...
	Protocol string
	// Task is the high-level task type: "chat", "messages", "text-to-image", ...
	Task string
	// Model is the model field being planned.  It starts as the raw model
	// from the request body and is replaced by the concrete model when
	// automatic routing selects one.
	Model string
	// RequestModel is the model ID the client actually asked for (for
	// example the "auto" virtual model).  It is preserved before automatic
	// routing rewrites Model so metrics can attribute the request to the
	// requested model while Model keeps naming the serving upstream model.
	RequestModel string
	// TenantID is the namespace UUID (nsUUID).
	TenantID string
	// UserID is the username.
	UserID string
	// PriorityScope is the server-side queue priority scope assigned to the
	// API key by its quota configuration ("high"/"low"; empty = unset, which
	// resolves to the high class — see AdmissionPriorityFromSource). It
	// is populated by the Extract phase from the request context and is the
	// sole source of the queue priority in the admission reservation queue.
	PriorityScope string
	// Streaming indicates whether the client requested a streaming response.
	Streaming bool
	// Headers are the HTTP request headers.
	Headers http.Header
	// ParsedBody holds the protocol-specific parsed request body
	// (e.g. *AnthropicMessagesRequest).  The Planner and Execute phase
	// type-assert this field to access protocol-specific fields.
	ParsedBody any
	// RequiredUpstreamID, when non-zero, forces the Planner's model
	// resolver to select the upstream with this ID.  Used by the
	// Responses protocol when the client supplies a previous_response_id
	// whose original upstream must be reused.  All other protocols leave
	// this as zero.
	RequiredUpstreamID int64
	// QueueWaitMs accumulates the time spent waiting in upstream admission
	// reservation queues across ALL plan rounds (including re-plans after a
	// reroute). The Planner fills it; the Orchestrator copies it onto the
	// RequestMetrics after the Plan phase so it lands in the per-request
	// metrics event.
	QueueWaitMs int64
	// QueueDepth is the maximum admission queue depth observed across ALL
	// plan rounds. The Orchestrator copies it onto the RequestMetrics.
	QueueDepth int64
}

// PromptTextProvider is implemented by protocol-specific request types that
// can extract a plain-text representation of the user's prompt for
// sensitive-content checking.  Each protocol's request type (e.g.
// AnthropicMessagesRequest) implements PromptText() so the Planner can
// obtain the text without type-switching on concrete types.
type PromptTextProvider interface {
	PromptText() string
}

// PromptText returns the prompt text from the parsed request body if it
// implements PromptTextProvider, otherwise an empty string.  This allows
// the Planner to perform sensitive-content checks without knowing the
// concrete protocol type.
func (m *RequestMetadata) PromptText() string {
	if m == nil || m.ParsedBody == nil {
		return ""
	}
	if p, ok := m.ParsedBody.(PromptTextProvider); ok {
		return p.PromptText()
	}
	return ""
}

// RequestPlan describes "how this request should be handled".  It is the
// output of the Plan phase and the input to the Adapt and Execute phases.
type RequestPlan struct {
	// ModelTarget is the resolved upstream model target.
	ModelTarget *ModelTarget
	// BackendURL is the URL to proxy the request to.
	BackendURL string
	// ClientProtocol is the API format the client called with ("chat" /
	// "responses" / "messages").  Attempt-level consumers (e.g. chat
	// fallback) use it to resolve fallback upstream URLs the same way the
	// planner resolved the primary one.  Empty when unknown.
	ClientProtocol string
	// RouteMode is the routing decision: "native", "adapter", or "disabled".
	RouteMode string
	// AdapterKind identifies which adapter to use when RouteMode == "adapter".
	AdapterKind string
	// UpstreamProtocol is the detected protocol of the upstream endpoint.
	UpstreamProtocol string
	// UpstreamCap describes the capabilities of the upstream protocol.
	UpstreamCap ProtocolCapability
	// AutoRoute records the automatic model selection when the client
	// requested the virtual model, and is nil for every other request.
	AutoRoute *AutoRouteDecision
	// BalanceOK indicates whether the balance check passed.
	BalanceOK bool
	// Safety holds the sensitive-content check result.  nil means no check
	// was performed.
	Safety *SafetyDecision
	// Admission holds the capacity-admission decision (nil when admission
	// did not apply, e.g. no upstream has an enabled CapacityPolicy). When
	// present, Decision.Lease identifies the acquired Redis lease; the
	// lease is released/finalized by the Orchestrator's safety-net defer and
	// the per-protocol usage commit path.
	Admission *AdmissionDecision
	// ErrorCode categorizes a Plan-phase error so the protocol handler can
	// choose the appropriate error response format.  It is only meaningful
	// when Plan returns an error.  See PlanErrorCategory constants.
	ErrorCode PlanErrorCategory
}

// PlanErrorCategory categorizes Plan-phase errors.
type PlanErrorCategory int

const (
	PlanErrUnknown PlanErrorCategory = iota
	PlanErrModelNotFound
	PlanErrModelUnavailable
	PlanErrInsufficientBalance
	PlanErrDisabled
	PlanErrSensitive
	PlanErrCapacityExceeded
	// PlanErrQueueTimeout marks a request that gave up waiting in an
	// upstream's admission reservation queue (QueueWaitSeconds exceeded).
	// Rendered as HTTP 408 rather than 429: the capacity did not reject the
	// request, the client's tolerance for waiting did.
	PlanErrQueueTimeout
	// PlanErrQueueCancelled marks a request that left the queue because the
	// CLIENT disconnected (mid-queue cancel). Rendered as the same 408 shape
	// as the admission_lease path; the response rarely reaches the client.
	PlanErrQueueCancelled
	PlanErrInternal
	// PlanErrQuotaExceeded marks a request rejected because the tenant's
	// active token plan subscription has exhausted the requested model's
	// allowance for the current period and its overage policy rejects
	// further use. It is rendered as HTTP 429. Deliberately appended after
	// PlanErrInternal so the existing constant values do not shift.
	PlanErrQuotaExceeded
	PlanErrRoutingInputTooLarge
	// PlanErrUpstreamPriorityDenied marks a request whose caller priority
	// (from user tags) admits no available upstream of the model — e.g. a
	// low-priority user with no priority-0 upstream configured. Rendered as
	// HTTP 403. Appended last so the existing constant values do not shift.
	PlanErrUpstreamPriorityDenied
)

// ModelTarget is the protocol-agnostic model target information produced by
// the Planner.  It mirrors handler.resolvedModelTarget but lives in the types
// package so it can be shared across handler subpackages without circular
// dependencies.
type ModelTarget struct {
	Model          *Model
	Upstream       commonType.UpstreamConfig
	Target         string
	Host           string
	ModelName      string
	AttemptTargets []commonType.UpstreamConfig
}

// SafetyDecision describes the result of a sensitive-content check.
type SafetyDecision struct {
	// IsSensitive is true when the input was flagged by the moderation policy.
	IsSensitive bool
	// Message is the block message to present to the client.
	Message string
}

// AdaptResult is the output of the Adapt phase.  It carries the transformed
// request body and the response writer that will intercept the upstream
// response for protocol conversion and usage extraction.
type AdaptResult struct {
	// Body is the marshalled request body to send to the upstream.
	Body []byte
	// Writer is the response writer that wraps the gin writer.  It must
	// implement http.ResponseWriter and http.Flusher.  It may optionally
	// implement Usage() (int64, int64) and Finalize() error for usage
	// extraction and response finalization.
	Writer HTTPResponseWriter
}

// HTTPResponseWriter is the minimal interface the reverse proxy needs to
// write responses.  It is satisfied by gin.ResponseWriter and by the
// protocol-specific adapter response writers.
type HTTPResponseWriter interface {
	http.ResponseWriter
	http.Flusher
}

// AutoRouteAlternatesRemaining reports how many ranked models are still
// held in reserve for this request.  The attempt loop consumes them as it
// tries them, so this is the live remaining budget rather than what the
// ranking originally offered.
func (p *RequestPlan) AutoRouteAlternatesRemaining() int {
	if p == nil || p.AutoRoute == nil {
		return 0
	}
	return len(p.AutoRoute.Alternates)
}

// TakeAutoRouteAlternate removes and returns the next ranked model to try,
// or nil when the request has exhausted its alternates.
func (p *RequestPlan) TakeAutoRouteAlternate() *AutoRouteCandidate {
	if p.AutoRouteAlternatesRemaining() == 0 {
		return nil
	}
	next := p.AutoRoute.Alternates[0]
	p.AutoRoute.Alternates = p.AutoRoute.Alternates[1:]
	return &next
}
