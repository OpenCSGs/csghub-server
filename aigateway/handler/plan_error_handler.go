package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// handleOpenAIPlanError renders Plan-phase errors in the standard OpenAI
// error format ({"error": {"code":..., "message":..., "type":...}}).
// It is shared across all OpenAI-compatible protocol handlers that go
// through the three-stage pipeline (rerank, embedding, image, audio,
// ocr, speech, video, responses, chat).
//
// The frontendURL is used for the insufficient-balance message which
// includes a recharge URL.  When frontendURL is empty the recharge link
// will be relative ("/settings/recharge-payment").
func handleOpenAIPlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error, frontendURL string) {
	if p != nil {
		switch p.ErrorCode {
		case types.PlanErrModelNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": types.Error{
				Code: "model_not_found", Message: err.Error(), Type: "not_found_error",
			}})
			return
		case types.PlanErrModelUnavailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": types.Error{
				Code: "model_unavailable", Message: err.Error(), Type: "server_error",
			}})
			return
		case types.PlanErrInsufficientBalance:
			c.JSON(http.StatusPaymentRequired, gin.H{"error": gin.H{
				"code":    "insufficient_balance",
				"message": insufficientBalanceMessage(frontendURL),
				"type":    "insufficient_balance",
			}})
			return
		case types.PlanErrUsageLimitExceeded:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"code":    "rate_limit_exceeded",
				"message": "Usage quota exceeded for current window",
				"type":    "rate_limit_error",
			}})
			return
		case types.PlanErrDisabled:
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "unsupported_feature", Message: fmt.Sprintf("this endpoint is not available for this model: %v", err), Type: "invalid_request_error",
			}})
			return
		case types.PlanErrSensitive:
			message := "content blocked due to safety policy"
			if p.Safety != nil && p.Safety.Message != "" {
				message = p.Safety.Message
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"code":    "content_policy_violation",
				"message": message,
				"type":    "invalid_request_error",
			}})
			return
		case types.PlanErrInternal:
			slog.ErrorContext(c.Request.Context(), "plan internal error",
				slog.String("model", meta.Model), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
				Code: "internal_error", Message: "an internal error occurred while processing the request", Type: "internal_error",
			}})
			return
		}
	}
	// Fallback: p is nil, or ErrorCode is PlanErrUnknown/anything else not
	// mapped above.  These correspond to unexpected/internal failures whose
	// raw message must not leak to the API consumer — render the same generic
	// response as the PlanErrInternal case.
	slog.ErrorContext(c.Request.Context(), "plan error",
		slog.String("model", meta.Model), slog.Any("error", err))
	c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
		Code: "internal_error", Message: "an internal error occurred while processing the request", Type: "internal_error",
	}})
}
