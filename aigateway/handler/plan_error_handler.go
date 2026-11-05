package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

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
	if handleRoutingInputPlanError(c, p, err) {
		return
	}
	if p != nil {
		switch p.ErrorCode {
		case types.PlanErrModelNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": types.Error{
				Code: types.ErrCodeModelNotFound, Message: err.Error(), Type: types.ErrTypeInvalidRequest,
			}})
			return
		case types.PlanErrModelUnavailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": types.Error{
				Code: types.ErrCodeModelUnavailable, Message: err.Error(), Type: types.ErrTypeServer,
			}})
			return
		case types.PlanErrInsufficientBalance:
			c.JSON(http.StatusPaymentRequired, gin.H{"error": types.Error{
				Code: types.ErrCodeInsufficientBalance, Message: insufficientBalanceMessage(frontendURL), Type: types.ErrTypeInsufficientBalance,
			}})
			return
		case types.PlanErrQuotaExceeded:
			// The tenant's own token plan allowance for this model is spent:
			// a client-facing 429, not an outage (which stays PlanErrInternal).
			c.JSON(http.StatusTooManyRequests, gin.H{"error": types.Error{
				Code: types.ErrCodeTokenPlanQuotaExceeded, Message: err.Error(), Type: types.ErrTypeRateLimit,
			}})
			return
		case types.PlanErrQueueTimeout:
			// The client's queue-wait tolerance expired, not the capacity
			// gate: 408, no Retry-After (the retry decision is the
			// client's; the queue position is already released).
			c.JSON(http.StatusRequestTimeout, gin.H{"error": types.Error{
				Code: types.AdmissionReasonQueueTimeout, Message: types.MsgQueueTimeout, Type: types.ErrTypeTimeout,
			}})
			return
		case types.PlanErrQueueCancelled:
			// The request left the queue because the CLIENT went away
			// (mid-queue disconnect): keep the same 408 shape as the
			// admission_lease rendering for consistency. In practice the
			// response rarely reaches the client; this covers the race
			// where the disconnect is observed only after the plan phase.
			c.JSON(http.StatusRequestTimeout, gin.H{"error": types.Error{
				Code: types.AdmissionReasonQueueCancelled, Message: types.MsgQueueCancelled, Type: types.ErrTypeTimeout,
			}})
			return
		case types.PlanErrCapacityExceeded:
			message := types.MsgCapacityExceeded
			retryAfter := int64(1)
			code := types.AdmissionReasonCapacityExceeded
			if p.Admission != nil {
				if p.Admission.Reason != "" {
					message = "model capacity exceeded: " + p.Admission.Reason
				}
				if p.Admission.Reason == types.AdmissionReasonQueueFull {
					code = types.AdmissionReasonQueueFull
				}
				if p.Admission.RetryAfterSeconds > 0 {
					retryAfter = p.Admission.RetryAfterSeconds
				}
			}
			// Retry-After is a retry hint, not a capacity guarantee.
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": types.Error{
				Code: code, Message: message, Type: types.ErrTypeRateLimit,
			}})
			return
		case types.PlanErrDisabled:
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: types.ErrCodeUnsupportedFeature, Message: fmt.Sprintf("this endpoint is not available for this model: %v", err), Type: types.ErrTypeInvalidRequest,
			}})
			return
		case types.PlanErrSensitive:
			message := types.MsgContentBlocked
			if p.Safety != nil && p.Safety.Message != "" {
				message = p.Safety.Message
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: types.ErrCodeContentPolicyViolation, Message: message, Type: types.ErrTypeInvalidRequest,
			}})
			return
		case types.PlanErrUpstreamPriorityDenied:
			// The caller's user-tag priority admits no available upstream of
			// this model: a client-facing 403, not an outage.
			c.JSON(http.StatusForbidden, gin.H{"error": types.Error{
				Code: types.ErrCodeUpstreamPriorityDenied, Message: err.Error(), Type: types.ErrTypePermission,
			}})
			return
		case types.PlanErrInternal:
			slog.ErrorContext(c.Request.Context(), "plan internal error",
				slog.String("model", meta.Model), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
				Code: types.ErrCodeInternal, Message: types.MsgInternalError, Type: types.ErrTypeServer,
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
		Code: types.ErrCodeInternal, Message: types.MsgInternalError, Type: types.ErrTypeServer,
	}})
}
