// Package decisions implements OpenAI Decisions (/v1/decisions) for the
// the AIGateway.  It follows the shared three-phase architecture:
//
//  1. Extract — parse the Decision request and extract identity.
//  2. Plan — (delegated to the shared Planner) resolve model target, check
//     balance/quota/safety.
//  3. Execute — proxy the request through the gateway's unified reverse
//     proxy, record metrics, usage, and the LLM trace.
//
// The protocol is a single synchronous request/response exchange: the client
// submits evidence plus typed questions, the upstream answers each question.
// There is no streaming variant.
//
// The handler implements both MetadataExtractor and ProtocolHandler.  The
// Orchestrator calls Extract → Plan → Execute in sequence.
package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// Handler handles OpenAI Decisions requests.
// It implements both MetadataExtractor and ProtocolHandler.
type Handler struct {
	Deps
}

// New creates a new decisions Handler with the given dependencies.
func New(deps Deps) *Handler {
	return &Handler{deps}
}

// Ensure Handler implements MetadataExtractor and ProtocolHandler.
var (
	_ plan.MetadataExtractor = (*Handler)(nil)
	_ plan.ProtocolHandler   = (*Handler)(nil)
)

// --- Phase 1: Extract ---

// Extract parses the Decision request and extracts identity information.  No
// business logic is performed here.
func (h *Handler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	req := &types.DecisionRequest{}
	if err := c.BindJSON(req); err != nil {
		writeBadRequest(c, fmt.Sprintf("invalid request body: %v", err))
		return nil, err
	}
	if err := req.Validate(); err != nil {
		writeBadRequest(c, err.Error())
		return nil, err
	}

	return &types.RequestMetadata{
		Protocol:      string(types.ProtocolDecisions),
		Task:          "decisions",
		Model:         req.Model,
		TenantID:      nsUUID,
		UserID:        username,
		PriorityScope: httpbase.GetAPIKeyPriorityScope(c),
		Streaming:     false,
		Headers:       c.Request.Header,
		ParsedBody:    req,
	}, nil
}

// --- Phase 3: Execute ---

// Execute forwards the request to the upstream through the unified reverse
// proxy and runs the common finalization: response validation, metrics,
// usage recording, and LLM trace.
func (h *Handler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	// Record resolved model on the preflight span and end it.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(meta.Model, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	// The planner only routes Decisions natively; adapter and disabled
	// modes are rejected during the Plan phase, so anything non-native here
	// is a routing bug. An empty mode is tolerated for callers whose plan
	// was produced without protocol routing (tests).
	if p.RouteMode != "" && p.RouteMode != string(protocol.ModeNative) {
		err := fmt.Errorf("unsupported route mode %q for the decisions protocol", p.RouteMode)
		writeError(c, http.StatusBadRequest, ErrTypeUnsupported, err.Error())
		return nil
	}
	if p.ModelTarget == nil {
		err := fmt.Errorf("model target is not resolved")
		writeError(c, http.StatusInternalServerError, ErrTypeInternal, err.Error())
		return nil
	}

	// Start LLM generation trace.
	requestID := commontrace.GetTraceIDInGinContext(c)
	traceCtx, recorder := h.startDecisionsTrace(c.Request.Context(), meta, p, requestID)
	c.Request = c.Request.WithContext(traceCtx)

	req := meta.ParsedBody.(*types.DecisionRequest)

	// Adapt: build the upstream body from the client's original bytes,
	// patching only the model name; the response writer then validates the
	// proxied response before anything reaches the client.
	body, err := upstreamRequestBody(req, p.ModelTarget.ModelName)
	if err != nil {
		writeError(c, http.StatusInternalServerError, ErrTypeInternal, "an internal error occurred while processing the request")
		finishLLMTraceWithError(recorder, err, types.TraceErrUpstreamError)
		h.finalizeAdmissionLease(c.Request.Context(), admissionLeaseFromPlan(p), nil)
		return nil
	}

	// Apply the upstream credential to the proxied request. No gateway-side
	// timeout is imposed: the reverse proxy forwards without a deadline and
	// the client owns its own timeout budget.
	if err := types.ApplyRequestAuthHeaders(c.Request.Header, p.ModelTarget.Upstream.AuthHeader); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth header",
			slog.String("model", p.ModelTarget.ModelName),
			slog.Any("error", err))
	}

	// Proxy the request; the buffered writer validates the response before
	// anything reaches the client.
	writer := newDecisionsResponseWriter(c.Writer)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	h.ProxyExecutor.ServeProxy(c, p.BackendURL, p.ModelTarget.Host, writer)
	writer.Finalize()

	usage := writer.Usage()
	if !writer.Succeeded() {
		if h.ErrorRecorder != nil && writer.Err() != nil {
			h.ErrorRecorder.SetMetricsError(c, writer.ErrorType(), writer.Err().Error())
		}
		finishLLMTraceWithError(recorder, writer.Err(), types.TraceErrUpstreamError)
		h.finalizeAdmissionLease(c.Request.Context(), admissionLeaseFromPlan(p), nil)
		return nil
	}

	// Build the trace output from the parsed response.
	traceOutput := traceOutputFromResponse(writer.Response())

	// Record token usage metrics (sync — before c.Next() returns).
	if h.MetricsRecorder != nil {
		h.MetricsRecorder.RecordTokenUsage(c, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	}

	// Async post-process: usage record, admission lease finalization, and trace.
	h.runPostProcessAsync(c.Request.Context(), postProcessInput{
		NSUUID:          meta.TenantID,
		TokenID:         httpbase.GetCurrentTokenID(c),
		Model:           modelOf(p),
		TargetModelName: targetModelName(p),
		Usage:           usage,
		Recorder:        recorder,
		Input:           decisionsTraceInput(req),
		Output:          traceOutput,
		ResponseModel:   writer.Response().Model,
		StatusCode:      writer.StatusCode(),
		AdmissionLease:  admissionLeaseFromPlan(p),
	})
	return nil
}

// upstreamRequestBody builds the upstream body while preserving client input
// and changing only the resolved model when necessary.
// Native requests keep the client's original bytes: only the model field is
// patched to the resolved upstream name, so every other field — including
// protocol additions unknown to the gateway — is forwarded exactly as sent.
// Requests built programmatically (no raw body, e.g. tests) keep the typed
// serialization path.
func upstreamRequestBody(req *types.DecisionRequest, upstreamModel string) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("decisions request is nil")
	}
	if len(req.RawBody) > 0 {
		// Fast path: no model rewrite needed — forward the client's bytes
		// without any JSON round trip.
		if upstreamModel == "" || req.ClientModel == upstreamModel {
			return req.RawBody, nil
		}
		return patchDecisionRawModel(req.RawBody, upstreamModel)
	}
	reqCopy := *req
	if upstreamModel != "" {
		reqCopy.Model = upstreamModel
	}
	body, err := json.Marshal(&reqCopy)
	if err != nil {
		return nil, fmt.Errorf("marshal decisions request: %w", err)
	}
	return body, nil
}

// patchDecisionRawModel rewrites the client's raw Decisions body for the
// upstream: only the model field is replaced with the resolved upstream
// name. Every other field is forwarded as the client sent it, so future
// protocol additions keep working without gateway changes.
// --- HandlePlanError ---

// HandlePlanError renders the protocol-specific error response for a
// Plan-phase rejection.  The RequestPlan's ErrorCode identifies the category.
func (h *Handler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	// Record preflight error if the span is still open.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		plan.SetPreflightTracer(c, nil)
	}

	if p != nil {
		switch p.ErrorCode {
		case types.PlanErrModelNotFound:
			writeError(c, http.StatusNotFound, ErrTypeModelNotFound, err.Error())
			return
		case types.PlanErrModelUnavailable:
			writeError(c, http.StatusServiceUnavailable, ErrTypeModelUnavailable, err.Error())
			return
		case types.PlanErrInsufficientBalance:
			writeError(c, http.StatusPaymentRequired, ErrTypeInsufficientBal,
				fmt.Sprintf("insufficient balance: %v", err))
			return
		case types.PlanErrQueueTimeout:
			// The client's queue-wait tolerance expired, not the capacity
			// gate: 408, no Retry-After.
			writeError(c, http.StatusRequestTimeout, ErrTypeTimeout,
				"queued request exceeded its queue wait time for model capacity")
			return
		case types.PlanErrQueueCancelled:
			// The request left the queue because the CLIENT went away
			// (mid-queue disconnect): same 408 shape as queue_timeout.
			writeError(c, http.StatusRequestTimeout, ErrTypeTimeout,
				"request left the admission queue before being served")
			return
		case types.PlanErrCapacityExceeded:
			message := "model capacity exceeded, please retry later"
			retryAfter := int64(1)
			if p.Admission != nil {
				if p.Admission.Reason != "" {
					message = "model capacity exceeded: " + p.Admission.Reason
				}
				if p.Admission.RetryAfterSeconds > 0 {
					retryAfter = p.Admission.RetryAfterSeconds
				}
			}
			// Retry-After is a retry hint, not a capacity guarantee.
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			writeError(c, http.StatusTooManyRequests, ErrTypeRateLimit, message)
			return
		case types.PlanErrDisabled:
			writeError(c, http.StatusBadRequest, ErrTypeUnsupported,
				fmt.Sprintf("/v1/decisions is not available for this model: %v", err))
			return
		case types.PlanErrInternal:
			writeError(c, http.StatusInternalServerError, ErrTypeInternal,
				"an internal error occurred while processing the request")
			return
		case types.PlanErrSensitive:
			message := "content blocked due to safety policy"
			if p.Safety != nil && p.Safety.Message != "" {
				message = p.Safety.Message
			}
			writeError(c, http.StatusBadRequest, ErrTypeContentPolicy, message)
			return
		}
	}
	slog.ErrorContext(c.Request.Context(), "decisions plan error",
		slog.String("model", meta.Model), slog.Any("error", err))
	writeError(c, http.StatusInternalServerError, ErrTypeInternal, err.Error())
}

// --- Trace and log capture ---

// startDecisionsTrace starts an LLM generation trace for Decisions.
// protocol.  Returns (ctx, nil) when tracing is disabled.
func (h *Handler) startDecisionsTrace(ctx context.Context, meta *types.RequestMetadata, p *types.RequestPlan, requestID string) (context.Context, GenerationRecorder) {
	if h.LLMTracer == nil || p.ModelTarget == nil || p.ModelTarget.Model == nil {
		return ctx, nil
	}
	req, _ := meta.ParsedBody.(*types.DecisionRequest)
	return h.LLMTracer.StartGeneration(ctx, types.GenerationStart{
		RequestID:     requestID,
		UserID:        meta.TenantID,
		Provider:      p.ModelTarget.Model.Provider,
		RequestModel:  meta.Model,
		ResolvedModel: p.ModelTarget.ModelName,
		Mode:          types.GenerationModeSync,
		Input:         decisionsTraceInput(req),
		Metadata: map[string]any{
			"aigateway.api":      "/v1/decisions",
			"aigateway.model_id": p.ModelTarget.Model.ID,
		},
	})
}

// decisionsTraceInput records the opaque input and questions in trace context
// without interpreting multimodal content.
func decisionsTraceInput(req *types.DecisionRequest) []types.GenerationMessage {
	if req == nil {
		return nil
	}
	return []types.GenerationMessage{{
		Role: "user",
		Parts: []types.GenerationPart{{
			Kind: "text",
			Text: string(req.Input) + "\nquestions=" + string(req.Questions),
		}},
	}}
}

// --- Post-processing ---

// runPostProcessAsync runs the post-processing goroutine after the upstream
// response completes.  It mirrors the anthropic package's post-process:
//   - LLM trace completion (usage, response, error status)
//   - Capacity admission lease finalization with the real usage
//   - Usage record (only on successful status)
//
// Decisions deliberately does not publish an LLM training log.
func (h *Handler) runPostProcessAsync(ctx context.Context, input postProcessInput) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in decisions post-process", slog.Any("panic", r))
				if input.Recorder != nil {
					input.Recorder.End()
				}
			}
		}()

		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), traceDeadline)
		defer cancel()

		// LLM trace completion.
		if input.Recorder != nil {
			recordDecisionTraceCompletion(input)
			input.Recorder.End()
		}

		// Finalize the capacity admission lease with the real usage
		// (idempotent with the Orchestrator's safety-net release).
		h.finalizeAdmissionLease(usageCtx, input.AdmissionLease, admissionUsageOf(input.Usage))

		// Record usage (only on successful status).
		if h.UsageRecorder != nil && input.Model != nil && input.Usage != nil && isSuccessfulStatus(input.StatusCode) {
			if err := h.UsageRecorder.RecordUsage(usageCtx, input.NSUUID, input.Model, input.TargetModelName, input.Usage.toUsage(), input.TokenID); err != nil {
				slog.ErrorContext(usageCtx, "failed to record token usage", slog.Any("error", err))
			}
		}
	}()
}

// recordDecisionTraceCompletion records the generation response and error status
// on the trace recorder.  The trace input/output come straight from the
// parsed protocol request and response, so the trace does not depend on the
// (chat-shaped) training-log capture.
func recordDecisionTraceCompletion(input postProcessInput) {
	provider := ""
	if input.Model != nil {
		provider = input.Model.Provider
	}
	responseModel := input.ResponseModel
	if responseModel == "" {
		responseModel = input.TargetModelName
	}
	input.Recorder.SetFirstChunk(types.GenerationFirstChunk{})
	input.Recorder.SetResponse(types.GenerationResponse{
		Provider:      provider,
		Model:         responseModel,
		ResponseModel: responseModel,
		Input:         input.Input,
		Output:        input.Output,
		ResponseID:    input.ResponseID,
		Metadata:      map[string]any{"aigateway.usage.compute_units": usageComputeUnits(input.Usage)},
	})
	if input.StatusCode >= http.StatusBadRequest {
		input.Recorder.SetError(fmt.Errorf("HTTP %d", input.StatusCode), types.TraceErrUpstreamError)
	}
	input.Recorder.SetUsage(types.TokenUsage{
		InputTokens:           usagePromptTokens(input.Usage),
		OutputTokens:          usageCompletionTokens(input.Usage),
		TotalTokens:           usageTotalTokens(input.Usage),
		CacheReadInputTokens:  cachedPromptTokens(input.Usage),
		CacheWriteInputTokens: cacheWritePromptTokens(input.Usage),
		ReasoningTokens:       reasoningTokens(input.Usage),
	})
}

// traceOutputFromResponse converts the parsed upstream answers into the
// trace output message.
func traceOutputFromResponse(resp *types.DecisionResponse) []types.GenerationMessage {
	if resp == nil {
		return nil
	}
	answersJSON, err := json.Marshal(resp.Answers)
	if err != nil {
		answersJSON = []byte("[]")
	}
	return []types.GenerationMessage{{
		Role: "assistant",
		Parts: []types.GenerationPart{{
			Kind: "text",
			Text: string(answersJSON),
		}},
	}}
}

// finalizeAdmissionLease releases the capacity admission lease with the real
// usage (nil usage = full reservation reclaim).  Safe to call with a nil
// finalizer or lease.
func (h *Handler) finalizeAdmissionLease(ctx context.Context, lease *types.AdmissionLease, usage *types.AdmissionUsage) {
	if h.AdmissionFinalizer == nil || lease == nil {
		return
	}
	h.AdmissionFinalizer.FinalizeCapacityAdmission(ctx, lease, usage)
}

func usagePromptTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.PromptTokens
}

func usageCompletionTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.CompletionTokens
}

func usageTotalTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.TotalTokens
}

func cachedPromptTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.CachedPromptTokens
}
func cacheWritePromptTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.CacheCreationPromptTokens
}
func reasoningTokens(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.ReasoningTokens
}

func usageComputeUnits(u *tokenUsage) int64 {
	if u == nil {
		return 0
	}
	return u.ComputeUnits
}

func admissionUsageOf(u *tokenUsage) *types.AdmissionUsage {
	if u == nil {
		return nil
	}
	return &types.AdmissionUsage{TotalTokens: u.TotalTokens}
}

func modelOf(p *types.RequestPlan) *types.Model {
	if p == nil || p.ModelTarget == nil {
		return nil
	}
	return p.ModelTarget.Model
}

func targetModelName(p *types.RequestPlan) string {
	if p == nil || p.ModelTarget == nil {
		return ""
	}
	return p.ModelTarget.ModelName
}

func admissionLeaseFromPlan(p *types.RequestPlan) *types.AdmissionLease {
	if p == nil || p.Admission == nil {
		return nil
	}
	return p.Admission.Lease
}
