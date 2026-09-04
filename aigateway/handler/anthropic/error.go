package anthropic

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Anthropic error type constants.
const (
	ErrTypeInvalidRequest    = "invalid_request_error"
	ErrTypeAuthentication    = "authentication_error"
	ErrTypePermission        = "permission_error"
	ErrTypeNotFound          = "not_found_error"
	ErrTypeRateLimit         = "rate_limit_error"
	ErrTypeOverloaded        = "overloaded_error"
	ErrTypeAPI               = "api_error"
	ErrTypeUnsupported       = "unsupported_capability"
)

// writeError sends an Anthropic-format error response.
// Anthropic errors use the shape:
//
//	{
//	  "type": "error",
//	  "error": {
//	    "type": "invalid_request_error",
//	    "message": "..."
//	  }
//	}
func writeError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// writeBadRequest is a convenience wrapper for 400 errors.
func writeBadRequest(c *gin.Context, message string) {
	writeError(c, http.StatusBadRequest, ErrTypeInvalidRequest, message)
}

// writeUnsupportedCapability returns a 400 when the upstream cannot satisfy
// a requested capability (e.g. prompt_caching on a Chat-only upstream).
func writeUnsupportedCapability(c *gin.Context, missing []string) {
	writeError(c, http.StatusBadRequest, ErrTypeUnsupported,
		"upstream does not support: "+joinStrings(missing, ", "))
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// mapUpstreamStatusToAnthropic maps an upstream HTTP status code to an
// Anthropic-compatible status code and error type.  Overloaded upstream
// errors (502/503/504) are mapped to 529 (Anthropic overloaded_error).
func mapUpstreamStatusToAnthropic(statusCode int) (int, string) {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return 529, ErrTypeOverloaded
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests, ErrTypeRateLimit
	case http.StatusUnauthorized:
		return http.StatusUnauthorized, ErrTypeAuthentication
	case http.StatusForbidden:
		return http.StatusForbidden, ErrTypePermission
	case http.StatusNotFound:
		return http.StatusNotFound, ErrTypeNotFound
	case http.StatusBadRequest:
		return http.StatusBadRequest, ErrTypeInvalidRequest
	default:
		if statusCode >= 500 {
			return 529, ErrTypeOverloaded
		}
		return statusCode, ErrTypeAPI
	}
}

// extractUpstreamErrorMessage attempts to extract a human-readable error
// message from an upstream error response body.  It tries OpenAI's
// {"error":{"message":"..."}} format first, then falls back to the raw body.
func extractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return "upstream returned an error with no response body"
	}
	// Try OpenAI error format: {"error":{"message":"..."}}
	var openaiErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &openaiErr); err == nil && openaiErr.Error.Message != "" {
		return openaiErr.Error.Message
	}
	// Try a plain {"error":"..."} or {"message":"..."} shape.
	var simpleErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &simpleErr); err == nil {
		if simpleErr.Error != "" {
			return simpleErr.Error
		}
		if simpleErr.Message != "" {
			return simpleErr.Message
		}
	}
	// Fall back to raw body (truncated to avoid sending large error payloads).
	if len(body) > 512 {
		return string(body[:512])
	}
	return string(body)
}

// writeAnthropicErrorJSON writes an Anthropic error response with the given
// status code, error type, and message.  Unlike writeAnthropicUpstreamError,
// it does not map the status code — it writes the caller's values directly.
func writeAnthropicErrorJSON(w gin.ResponseWriter, status int, errType, message string) {
	hdr := w.Header()
	hdr.Set("Content-Type", "application/json")
	hdr.Del("Content-Length")
	hdr.Del("Content-Encoding")
	hdr.Del("Transfer-Encoding")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// writeAnthropicUpstreamError converts an upstream error response to
// Anthropic error format and writes it to the gin writer.
func writeAnthropicUpstreamError(w gin.ResponseWriter, statusCode int, body []byte) {
	anthropicStatus, errType := mapUpstreamStatusToAnthropic(statusCode)
	message := extractUpstreamErrorMessage(body)
	payload := gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	}
	// Reset headers that may have been set by the proxy or SSE setup
	// (e.g. Content-Length: 0 from the upstream 404, or text/event-stream
	// from the streaming prelude).  gin.ResponseWriter.Header() returns
	// the live header map, so we can mutate it before WriteHeader.
	hdr := w.Header()
	hdr.Set("Content-Type", "application/json")
	hdr.Del("Content-Length")
	hdr.Del("Content-Encoding")
	hdr.Del("Transfer-Encoding")
	w.WriteHeader(anthropicStatus)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeAnthropicStreamError writes an Anthropic error event into an SSE stream.
// This is used when an upstream error occurs mid-stream.
func writeAnthropicStreamError(w gin.ResponseWriter, statusCode int, body []byte) {
	_, errType := mapUpstreamStatusToAnthropic(statusCode)
	message := extractUpstreamErrorMessage(body)
	payload := gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	}
	jsonData, _ := json.Marshal(payload)
	_, _ = w.Write([]byte("event: error\ndata: "))
	_, _ = w.Write(jsonData)
	_, _ = w.Write([]byte("\n\n"))
	w.Flush()
}
