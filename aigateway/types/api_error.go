package types

// Error represents an error that originates from the API, i.e. when a request is
// made and the API returns a response with a HTTP status code. Other errors are
// not wrapped by this SDK.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	Type    string `json:"type"`
}

// Error "type" field vocabulary shared by every aigateway protocol surface
// (OpenAI-compatible, Anthropic Messages, SystemOne). These are the wire
// values clients match on; change them only with a compatibility reason.
const (
	ErrTypeInvalidRequest        = "invalid_request_error"
	ErrTypeServer                = "server_error"
	ErrTypeRateLimit             = "rate_limit_error"
	ErrTypeTimeout               = "timeout_error"
	ErrTypeAPI                   = "api_error"
	ErrTypeAuthentication        = "authentication_error"
	ErrTypePermission            = "permission_error"
	ErrTypeNotFound              = "not_found_error"
	ErrTypeOverloaded            = "overloaded_error"
	ErrTypeServiceUnavailable    = "service_unavailable_error"
	ErrTypeUnsupportedCapability = "unsupported_capability"
	// ErrTypeInsufficientBalance shares its wire value with
	// ErrCodeInsufficientBalance: the 402 insufficient-balance response
	// carries it as both the code and the type.
	ErrTypeInsufficientBalance = "insufficient_balance"
)

// Error "code" field vocabulary. A few code values are also rendered as the
// "type" by the SystemOne surface (whose renderer mirrors code into type);
// the jev package aliases these constants for that reason. Values are wire
// contract; change them only with a compatibility reason.
const (
	ErrCodeModelNotFound        = "model_not_found"
	ErrCodeNotFound             = "not_found"
	ErrCodeRoutingInputTooLarge = "routing_input_too_large"
	ErrCodeModelUnavailable     = "model_unavailable"
	ErrCodeInternal             = "internal_error"
	ErrCodeInternalServer       = "internal_server_error"
	ErrCodeInsufficientBalance  = "insufficient_balance"
	// Capacity-admission rejections propagate the AdmissionReason* values
	// (queue_full, queue_timeout, queue_cancelled, capacity_exceeded) into
	// the code field; AdmissionReason is their single definition point.
	ErrCodeContentPolicyViolation      = "content_policy_violation"
	ErrCodeUnsupportedFeature          = "unsupported_feature"
	ErrCodeTokenPlanQuotaExceeded      = "token_plan_quota_exceeded"
	ErrCodeModeration                  = "moderation_error"
	ErrCodeUpstreamResponse            = "upstream_response_error"
	ErrCodeTransform                   = "transform_error"
	ErrCodeUnsupportedModel            = "unsupported_model"
	ErrCodeModelTaskMismatch           = "model_task_mismatch"
	ErrCodeInsufficientPermissions     = "insufficient_permissions"
	ErrCodeModelPriceNotConfigured     = "model_price_not_configured"
	ErrCodeModelNotRunning             = "model_not_running"
	ErrCodeRequiredUpstreamUnavailable = "required_upstream_unavailable"
	// ErrCodeUpstreamPriorityDenied reports that the caller's user priority
	// (from their user tags) does not admit them to any available upstream
	// channel of the model (e.g. a low-priority user with no low-priority
	// upstream configured).
	ErrCodeUpstreamPriorityDenied   = "upstream_priority_denied"
	ErrCodeResponseRouteUnavailable = "response_route_unavailable"
	ErrCodeUpstreamResponseInvalid  = "upstream_response_invalid"
	ErrCodeModelPriceMisconfigured  = "model_price_misconfigured"
	ErrCodeClusterNotFound          = "cluster_not_found"
	ErrCodeInvalidResponseID        = "invalid_response_id"
	ErrCodeResponseIDForbidden      = "response_id_forbidden"
	// ErrCodeInvalidRequest preserves paths that historically repeat the
	// type name in the code field; new errors should use a specific code.
	ErrCodeInvalidRequest = "invalid_request_error"
)

// Shared error messages rendered by more than one surface. Single-site
// messages stay at their call site; move one here when it is duplicated.
const (
	MsgInternalError    = "an internal error occurred while processing the request"
	MsgCapacityExceeded = "model capacity exceeded, please retry later"
	MsgQueueTimeout     = "queued request exceeded its queue wait time for model capacity"
	MsgQueueCancelled   = "request left the admission queue before being served"
	MsgContentBlocked   = "content blocked due to safety policy"
	MsgSensitiveContent = "Input data may contain inappropriate content."
	MsgModelEmpty       = "Model cannot be empty"
	MsgInputEmpty       = "Input cannot be empty"
	MsgQueryEmpty       = "Query cannot be empty"
	MsgDocumentsEmpty   = "Documents cannot be empty"
)
