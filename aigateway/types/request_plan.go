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
	// Model is the raw model field from the request body (before resolution).
	Model string
	// TenantID is the namespace UUID (nsUUID).
	TenantID string
	// UserID is the username.
	UserID string
	// APIKeyID is the API key identifier.
	APIKeyID string
	// Streaming indicates whether the client requested a streaming response.
	Streaming bool
	// Headers are the HTTP request headers.
	Headers http.Header
	// ParsedBody holds the protocol-specific parsed request body
	// (e.g. *AnthropicMessagesRequest).  The Planner and Execute phase
	// type-assert this field to access protocol-specific fields.
	ParsedBody any
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
	// RouteMode is the routing decision: "native", "adapter", or "disabled".
	RouteMode string
	// AdapterKind identifies which adapter to use when RouteMode == "adapter".
	AdapterKind string
	// UpstreamProtocol is the detected protocol of the upstream endpoint.
	UpstreamProtocol string
	// UpstreamCap describes the capabilities of the upstream protocol.
	UpstreamCap ProtocolCapability
	// BalanceOK indicates whether the balance check passed.
	BalanceOK bool
	// Safety holds the sensitive-content check result.  nil means no check
	// was performed.
	Safety *SafetyDecision
	// UsageLimitOK indicates whether the usage-limit check passed.
	UsageLimitOK bool
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
	PlanErrUsageLimitExceeded
	PlanErrDisabled
	PlanErrSensitive
	PlanErrInternal
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
