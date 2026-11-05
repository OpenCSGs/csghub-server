package jev

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// Jev error type constants, aliased onto the shared aigateway vocabulary so
// the wire values have a single definition point.  System One has no
// documented error envelope of its own, so the gateway renders errors in the
// OpenAI-compatible shape used across its other endpoints:
// {"error": {"code", "message", "type"}} — and this renderer mirrors the
// code value into the type, so the specific-value aliases below point at the
// shared code vocabulary.
const (
	ErrTypeInvalidRequest   = types.ErrTypeInvalidRequest
	ErrTypeAuthentication   = types.ErrTypeAuthentication
	ErrTypePermission       = types.ErrTypePermission
	ErrTypeNotFound         = types.ErrTypeNotFound
	ErrTypeRateLimit        = types.ErrTypeRateLimit
	ErrTypeTimeout          = types.ErrTypeTimeout
	ErrTypeAPI              = types.ErrTypeAPI
	ErrTypeUnsupported      = types.ErrCodeUnsupportedFeature
	ErrTypeContentPolicy    = types.ErrCodeContentPolicyViolation
	ErrTypeInsufficientBal  = types.ErrCodeInsufficientBalance
	ErrTypeInternal         = types.ErrCodeInternal
	ErrTypeModelNotFound    = types.ErrCodeModelNotFound
	ErrTypeModelUnavailable = types.ErrCodeModelUnavailable
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
