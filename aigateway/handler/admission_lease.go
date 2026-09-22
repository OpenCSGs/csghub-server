package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// Capacity admission lease helpers shared by the chat and responses
// execution paths.
//
// Lifecycle per request:
//
//	Plan phase   — the admission checker acquires a lease for the primary
//	               upstream and stores it on plan.Admission.
//	Per attempt  — ensureAdmissionForAttempt reuses the lease when the
//	               attempt targets the same upstream, or finalizes the
//	               previous lease and acquires a pinned one for the fallback
//	               upstream.
//	AfterExecute — the Orchestrator's safety-net defer finalizes the lease
//	               that is still on the plan (release only).
//	Usage commit — the async post-process finalizes the lease with the real
//	               usage (commit + release). Both paths are idempotent.
func ensureAdmissionForAttempt(ctx context.Context, h *OpenAIHandlerImpl, p *types.RequestPlan, modelTarget *resolvedModelTarget) error {
	if p == nil || modelTarget == nil || modelTarget.Model == nil {
		return nil
	}
	var prevLease *types.AdmissionLease
	if p.Admission != nil {
		prevLease = p.Admission.Lease
	}
	// Reuse the plan-phase lease when this attempt targets the same
	// upstream, so a logical request does not consume RPM twice.
	if prevLease != nil && prevLease.UpstreamID == modelTarget.Upstream.ID {
		return nil
	}
	estimatedTokens := int64(0)
	if prevLease != nil {
		estimatedTokens = prevLease.ReservedTokens
	}
	// Fallback acquire: pin the target upstream and atomically
	// check+acquire its lease. Never re-runs the initial selection (which
	// could re-select the upstream that just failed) and never trusts the
	// plan-phase candidate snapshot — other replicas may have acquired the
	// remaining capacity in between.
	decision := h.openaiComponent.AcquireCapacityAdmission(
		ctx, modelTarget.Model, modelTarget.Upstream.ID, estimatedTokens,
	)
	// The previous attempt's lease is finalized regardless of the outcome:
	// this attempt either moves to the new upstream's lease or stops here.
	finalizeAdmissionLease(ctx, h, prevLease, nil)
	if p.Admission != nil {
		p.Admission.Lease = nil
	}
	if decision == nil {
		// The fallback upstream has no enabled CapacityPolicy: it proceeds
		// unprotected, consistent with the plan-phase skip semantics.
		return nil
	}
	if decision.Action != types.AdmissionAdmit {
		slog.WarnContext(ctx, "admission fallback acquire rejected",
			slog.String("model", modelTarget.Model.ID),
			slog.Int64("upstream_id", modelTarget.Upstream.ID),
			slog.String("reason", decision.Reason),
			admissionTokenAttr(decision))
		return &types.AdmissionDeniedError{Decision: decision}
	}
	slog.InfoContext(ctx, "admission fallback lease acquired",
		slog.String("model", modelTarget.Model.ID),
		slog.Int64("upstream_id", decision.SelectedUpstreamID),
		admissionTokenAttr(decision))
	p.Admission = decision
	return nil
}

// finalizeAdmissionLease finalizes an admission lease, swallowing errors —
// lease release is best-effort: the Redis expired-lease cleanup is the
// safety net when this fails. When the CLIENT already aborted the attempt
// (ctx canceled / client disconnect → HTTP 499), the release also removes
// the lease's own RPM window entry: an aborted request never reaches the
// upstream's inference path, so keeping its RPM accounting would rate-lock
// a legitimate client retry for the rest of the sliding window.
func finalizeAdmissionLease(ctx context.Context, h *OpenAIHandlerImpl, lease *types.AdmissionLease, usage *token.Usage) {
	if lease == nil || lease.Token == "" {
		return
	}
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if clientCanceledAttempt(ctx, 0) {
		h.openaiComponent.FinalizeCanceledCapacityAdmission(finalizeCtx, lease, usage)
		return
	}
	h.openaiComponent.FinalizeCapacityAdmission(finalizeCtx, lease, usage)
}

// clientCanceledAttempt reports whether the CLIENT aborted the attempt
// (context canceled / client disconnect, rendered as HTTP 499). The
// Orchestrator's release path uses this to also drop the lease's RPM window
// entry. Server-side failures keep the immutable RPM accounting.
func clientCanceledAttempt(ctx context.Context, statusCode int) bool {
	if statusCode == 499 { // nginx-style client-closed-request
		return true
	}
	return errors.Is(ctx.Err(), context.Canceled)
}

// admissionTokenAttr returns the log attribute tying a decision to its
// admission lease token (nil-safe). Search logs by request ID (trace_id) to
// find the token, by admission_token to find the request.
func admissionTokenAttr(decision *types.AdmissionDecision) slog.Attr {
	if decision == nil || decision.Lease == nil {
		return slog.String("admission_token", "")
	}
	return slog.String("admission_token", decision.Lease.Token)
}

// admissionLeaseFromPlan extracts the current admission lease from the plan.
func admissionLeaseFromPlan(p *types.RequestPlan) *types.AdmissionLease {
	if p == nil || p.Admission == nil {
		return nil
	}
	return p.Admission.Lease
}

// admissionStatusForReason maps an admission rejection reason to its HTTP
// status and error code. Queue timeout and client cancellation are 408s —
// the capacity gate did not reject the request, the wait did — while
// queue_full is a bounded-backpressure 429. Everything else keeps the
// capacity_exceeded 429.
func admissionStatusForReason(reason string) (int, string) {
	switch reason {
	case types.AdmissionReasonQueueTimeout:
		return http.StatusRequestTimeout, "queue_timeout"
	case types.AdmissionReasonQueueCancelled:
		return http.StatusRequestTimeout, "queue_cancelled"
	case types.AdmissionReasonQueueFull:
		return http.StatusTooManyRequests, "queue_full"
	default:
		return http.StatusTooManyRequests, "capacity_exceeded"
	}
}

// handleAdmissionDenied renders a capacity admission rejection as a 429/408
// response (stream-aware, matching the usage-limit rendering). The status
// comes from admissionStatusForReason; the Retry-After header is a retry
// hint, not a capacity guarantee, and is only set on 429 responses. Returns
// false when err is not an admission denial.
func (h *OpenAIHandlerImpl) handleAdmissionDenied(c *gin.Context, isStream bool, err error) bool {
	var denied *types.AdmissionDeniedError
	if !errors.As(err, &denied) {
		return false
	}
	message := "model capacity exceeded, please retry later"
	retryAfter := int64(1)
	reason := ""
	if denied.Decision != nil {
		reason = denied.Decision.Reason
		if reason != "" {
			message = "model capacity exceeded: " + reason
		}
		if denied.Decision.RetryAfterSeconds > 0 {
			retryAfter = denied.Decision.RetryAfterSeconds
		}
	}
	status, code := admissionStatusForReason(reason)
	if reason == types.AdmissionReasonQueueTimeout {
		message = "queued request exceeded its queue wait time for model capacity"
	} else if reason == types.AdmissionReasonQueueCancelled {
		message = "request left the admission queue before being served"
	}
	slog.WarnContext(c.Request.Context(), "capacity admission denied",
		slog.String("model", c.Param("model")),
		slog.String("reason", message),
		slog.Int64("retry_after", retryAfter),
		admissionTokenAttr(denied.Decision))

	payload := gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
			"type":    "rate_limit_error",
		},
	}
	if code == "queue_timeout" || code == "queue_cancelled" {
		payload["error"].(gin.H)["type"] = "timeout_error"
	}
	if status == http.StatusTooManyRequests {
		c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	}
	if isStream {
		errorChunkJSON, _ := json.Marshal(payload)
		if _, writeErr := c.Writer.Write([]byte("data: " + string(errorChunkJSON) + "\n\ndata: [DONE]\n\n")); writeErr != nil {
			slog.Error("failed to write admission denied error to stream", "error", writeErr)
		}
		c.Writer.Flush()
		return true
	}
	c.JSON(status, payload)
	return true
}
