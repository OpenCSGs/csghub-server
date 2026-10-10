package jev

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// Jev error type constants.  System One has no documented error envelope of
// its own, so the gateway renders errors in the OpenAI-compatible shape used
// across its other endpoints: {"error": {"code", "message", "type"}}.
const (
	ErrTypeInvalidRequest   = "invalid_request_error"
	ErrTypeAuthentication   = "authentication_error"
	ErrTypePermission       = "permission_error"
	ErrTypeNotFound         = "not_found_error"
	ErrTypeRateLimit        = "rate_limit_error"
	ErrTypeTimeout          = "timeout_error"
	ErrTypeAPI              = "api_error"
	ErrTypeUnsupported      = "unsupported_feature"
	ErrTypeContentPolicy    = "content_policy_violation"
	ErrTypeInsufficientBal  = "insufficient_balance"
	ErrTypeInternal         = "internal_error"
	ErrTypeModelNotFound    = "model_not_found"
	ErrTypeModelUnavailable = "model_unavailable"
)

// writeError sends a gateway error response in the OpenAI-compatible shape.
func writeError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{"error": types.Error{
		Code:    errType,
		Message: message,
		Type:    errType,
	}})
}

// writeBadRequest is a convenience wrapper for 400 errors.
func writeBadRequest(c *gin.Context, message string) {
	writeError(c, http.StatusBadRequest, ErrTypeInvalidRequest, message)
}

// mapUpstreamStatus maps an upstream HTTP status to the client-facing status
// and error type.  Upstream 4xx problems are the caller's problem and are
// passed through with a specific type; 5xx means the upstream (or the
// gateway's credentials on its behalf) is broken and surfaces as 502.
func mapUpstreamStatus(statusCode int) (int, string) {
	switch statusCode {
	case http.StatusBadRequest:
		return http.StatusBadRequest, ErrTypeInvalidRequest
	case http.StatusUnauthorized:
		return http.StatusUnauthorized, ErrTypeAuthentication
	case http.StatusForbidden:
		return http.StatusForbidden, ErrTypePermission
	case http.StatusNotFound:
		return http.StatusNotFound, ErrTypeNotFound
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests, ErrTypeRateLimit
	default:
		if statusCode >= 500 {
			return http.StatusBadGateway, ErrTypeAPI
		}
		return statusCode, ErrTypeAPI
	}
}
