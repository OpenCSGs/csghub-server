// Package anthropic implements the Anthropic Messages API (/v1/messages) for the
// AIGateway.  It follows the three-phase architecture:
//
//  1. Extract — parse the Anthropic Messages request and extract identity.
//  2. Plan — (delegated to the shared Planner) resolve model target, check
//     balance/quota/safety.
//  3. Execute — adapt the request for the upstream protocol (native = identity,
//     adapter = protocol transform), proxy, finalize, record metrics, usage,
//     LLM trace, and LLM training log.
//
// The handler implements both MetadataExtractor and ProtocolHandler.  The
// Orchestrator calls Extract → Plan → Execute in sequence.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	commontypes "opencsg.com/csghub-server/common/types"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// Handler handles Anthropic Messages API requests.
// It implements both MetadataExtractor and ProtocolHandler.
type Handler struct {
	Deps
}

// New creates a new anthropic Handler with the given dependencies.
func New(deps Deps) *Handler {
	return &Handler{deps}
}

// Ensure Handler implements MetadataExtractor and ProtocolHandler.
var (
	_ plan.MetadataExtractor = (*Handler)(nil)
	_ plan.ProtocolHandler   = (*Handler)(nil)
)

// --- Phase 1: Extract ---

// Extract parses the Anthropic Messages request and extracts identity
// information.  No business logic is performed here.
func (h *Handler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	req := &types.AnthropicMessagesRequest{}
	if err := c.BindJSON(req); err != nil {
		writeBadRequest(c, fmt.Sprintf("invalid request body: %v", err))
		return nil, err
	}
	if err := req.Validate(); err != nil {
		writeBadRequest(c, err.Error())
		return nil, err
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolMessages),
		Task:       "messages",
		Model:      req.Model,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  req.Stream,
		Headers:    c.Request.Header,
		ParsedBody: req,
	}, nil
}

// --- Phase 3: Execute ---

// Execute adapts the request for the upstream protocol, then runs the
// common execution skeleton: set body, apply auth headers, set SSE headers,
// proxy, finalize, record usage/metrics, start/finish LLM trace, publish
// LLM training log, and commit usage limit.
//
// If the adapt step fails the error is rendered inline and Execute returns nil.
func (h *Handler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	// Record resolved model on the preflight span and end it.
	if pt := GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(meta.Model, p.ModelTarget)
		pt.End()
		SetPreflightTracer(c, nil)
	}

	// Start LLM generation trace.
	requestID := commontrace.GetTraceIDInGinContext(c)
	traceCtx, generationRecorder := h.startMessagesTrace(c.Request.Context(), c.Request.Header, meta, p, requestID)
	c.Request = c.Request.WithContext(traceCtx)

	// Create LLM log recorder for training log capture.
	logCapture := h.createLogCapture(c.Request.Context(), meta, p, requestID)

	adapted, err := h.adapt(c, meta, p, logCapture)
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		// Check for unsupported capability errors (from checkAdapterCapabilities).
		if missing := extractUnsupportedCapabilities(err); len(missing) > 0 {
			writeUnsupportedCapability(c, missing)
			return nil
		}
		writeError(c, http.StatusBadRequest, ErrTypeInvalidRequest, err.Error())
		return nil
	}

	// 1. Set the request body.
	c.Request.Body = io.NopCloser(bytes.NewReader(adapted.Body))
	c.Request.ContentLength = int64(len(adapted.Body))

	// 2. Apply auth headers.
	if p.ModelTarget != nil && p.ModelTarget.Upstream.AuthHeader != "" {
		if err := types.ApplyRequestAuthHeaders(c.Request.Header, p.ModelTarget.Upstream.AuthHeader); err != nil {
			slog.WarnContext(c.Request.Context(), "invalid auth header",
				slog.String("model", p.ModelTarget.ModelName),
				slog.Any("error", err))
		}
	}

	// 3. Record model target metrics (success path).
	if h.MetricsRecorder != nil {
		h.MetricsRecorder.SetModelTarget(c, meta.Model, p.ModelTarget, meta.Streaming)
	}

	// 4. Set SSE headers for streaming.
	if meta.Streaming {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
	}

	// 5. Execute the proxy request.
	host := ""
	if p.ModelTarget != nil {
		host = p.ModelTarget.Host
	}
	h.ProxyExecutor.ServeProxy(c, p.BackendURL, host, adapted.Writer)

	// 6. Finalize the writer.
	if f, ok := adapted.Writer.(finalizer); ok {
		if err := f.Finalize(); err != nil {
			slog.WarnContext(c.Request.Context(), "response writer finalize error",
				slog.Any("error", err))
		}
	}

	// 7. Extract usage.
	var usage tokenUsage
	if up, ok := adapted.Writer.(usageProvider); ok {
		usage = up.Usage()
	}

	// 8. Record token usage metrics (sync — before c.Next() returns).
	if h.MetricsRecorder != nil {
		h.MetricsRecorder.RecordTokenUsage(c, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	}

	// 9. Async post-process: usage record, usage limit commit, LLM trace
	//    completion, and LLM training log publishing.
	statusCode := 200
	if sw, ok := adapted.Writer.(statusProvider); ok {
		statusCode = sw.StatusCode()
	}
	h.runPostProcessAsync(c.Request.Context(), postProcessInput{
		NSUUID:          meta.TenantID,
		ApiKey:          httpbase.GetAccessToken(c),
		Model:           p.ModelTarget.Model,
		TargetModelName: p.ModelTarget.ModelName,
		Usage:           &usage,
		LogCapture:      logCapture,
		Trace: tracePostProcessInput{
			Recorder:   generationRecorder,
			Completion: generationRecorder != nil,
			Stream:     meta.Streaming,
			StatusCode: statusCode,
		},
		StatusCode: statusCode,
	})

	return nil
}

// finalizer is implemented by response writers that need a Finalize step
// after the proxy completes (e.g. to extract usage or convert the response
// body).
type finalizer interface {
	Finalize() error
}

// usageProvider is implemented by response writers that can report token
// usage after the response is finalized.
type usageProvider interface {
	Usage() tokenUsage
}

// statusProvider is implemented by response writers that can report their
// final HTTP status code.
type statusProvider interface {
	StatusCode() int
}

// startMessagesTrace starts an LLM generation trace for the Messages protocol.
// Returns (ctx, nil) when tracing is disabled.
func (h *Handler) startMessagesTrace(ctx context.Context, headers http.Header, meta *types.RequestMetadata, p *types.RequestPlan, requestID string) (context.Context, GenerationRecorder) {
	if h.LLMTracer == nil || p.ModelTarget == nil || p.ModelTarget.Model == nil {
		return ctx, nil
	}
	mode := types.GenerationModeSync
	if meta.Streaming {
		mode = types.GenerationModeStream
	}
	req := meta.ParsedBody.(*types.AnthropicMessagesRequest)
	return h.LLMTracer.StartGeneration(ctx, types.GenerationStart{
		RequestID:      requestID,
		ConversationID: extractMessagesSessionID(headers),
		UserID:         meta.TenantID,
		Provider:       p.ModelTarget.Model.Provider,
		RequestModel:   meta.Model,
		ResolvedModel:  p.ModelTarget.ModelName,
		Mode:           mode,
		Tools:          messagesTraceTools(req),
		ToolCount:      len(req.Tools),
		MaxTokens:      messagesTraceMaxTokens(req),
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		Metadata: map[string]any{
			"aigateway.api":      "/v1/messages",
			"aigateway.model_id": p.ModelTarget.Model.ID,
		},
	})
}

// createLogCapture creates an LLM log recorder for training log capture.
// Returns nil when the factory is not configured.
func (h *Handler) createLogCapture(ctx context.Context, meta *types.RequestMetadata, p *types.RequestPlan, requestID string) LLMLogRecorder {
	if h.LLMLogRecorderFactory == nil || p.ModelTarget == nil {
		return nil
	}
	req := meta.ParsedBody.(*types.AnthropicMessagesRequest)
	logReq, err := anthropicRequestToLLMLogRequest(req)
	if err != nil {
		slog.WarnContext(ctx, "failed to convert request for llmlog capture", slog.Any("error", err))
		logReq = commontypes.LLMLogRequest{Stream: req.Stream}
	}
	recorder, err := h.LLMLogRecorderFactory.NewLLMLogRecorder(
		requestID,
		p.ModelTarget.ModelName,
		meta.TenantID,
		logReq,
		map[string]any{
			"source":   "aigateway",
			"api":      "/v1/messages",
			"stream":   req.Stream,
			"provider": p.ModelTarget.Model.Provider,
			"svc_name": p.ModelTarget.Model.SvcName,
		},
	)
	if err != nil {
		slog.WarnContext(ctx, "failed to initialize llmlog training capture", slog.Any("error", err))
		return nil
	}
	return recorder
}

// runPostProcessAsync runs the post-processing goroutine after the upstream
// response completes.  It mirrors handler.runChatPostProcessAsync:
//   - LLM trace completion (usage, response, error status)
//   - Usage limit commit
//   - Usage record (only on successful status)
//   - LLM training log publishing
func (h *Handler) runPostProcessAsync(ctx context.Context, input postProcessInput) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in messages post-process", slog.Any("panic", r))
				if input.Trace.Recorder != nil {
					input.Trace.Recorder.End()
				}
			}
		}()

		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		// LLM trace completion.
		if input.Trace.Recorder != nil {
			var inputMsgs, outputMsgs []types.GenerationMessage
			var traceInfo commontypes.LLMLogTraceInfo
			if input.LogCapture != nil {
				in, out := input.LogCapture.Messages()
				inputMsgs = llmlogMessagesToGenerationMessages(in)
				outputMsgs = llmlogMessagesToGenerationMessages(out)
				traceInfo = input.LogCapture.TraceInfo()
			}
			recordMessagesTraceCompletion(input.Trace, input.Model, input.TargetModelName, input.Usage, inputMsgs, outputMsgs, traceInfo)
			input.Trace.Recorder.End()
		}

		// Commit usage limit.
		if h.UsageLimiter != nil && input.Model != nil {
			if err := h.commitUsageLimitSync(usageCtx, input.NSUUID, input.Model, input.Usage); err != nil {
				slog.ErrorContext(usageCtx, "failed to commit usage limit", slog.Any("error", err))
			}
		}

		// Record usage (only on successful status).
		if h.UsageRecorder != nil && input.Model != nil && input.Usage != nil && isSuccessfulStatus(input.StatusCode) {
			if err := h.UsageRecorder.RecordUsage(usageCtx, input.NSUUID, input.Model, input.TargetModelName, input.Usage.PromptTokens, input.Usage.CompletionTokens, input.Usage.CachedPromptTokens, input.Usage.CacheCreationPromptTokens, input.ApiKey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record token usage", slog.Any("error", err))
			}
		}

		// Publish LLM training log.
		if h.LLMLogPublisher != nil && input.LogCapture != nil {
			record, recordErr := input.LogCapture.Record()
			if recordErr != nil {
				// This can happen when the upstream returns an empty response
				// (e.g. no SSE events).  Log as a warning rather than an error
				// since it is not actionable.
				slog.WarnContext(usageCtx, "no llmlog training record to publish", slog.Any("error", recordErr))
				return
			}
			if record == nil {
				return
			}
			payload, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				slog.ErrorContext(usageCtx, "failed to marshal llmlog training record", slog.Any("error", marshalErr))
				return
			}
			if publishErr := h.LLMLogPublisher.PublishTrainingLog(payload); publishErr != nil {
				slog.ErrorContext(usageCtx, "failed to publish llmlog training record", slog.Any("error", publishErr))
			}
		}
	}()
}

func (h *Handler) commitUsageLimitSync(ctx context.Context, nsUUID string, model *types.Model, usage *tokenUsage) error {
	return h.UsageLimiter.CommitUsageLimitFromUsage(ctx, nsUUID, model, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, usage.CacheCreationPromptTokens)
}

// adapt transforms the request for the upstream protocol and creates the
// response writer.  This is the protocol-specific part of Execute.
func (h *Handler) adapt(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, logCapture LLMLogRecorder) (*types.AdaptResult, error) {
	req := meta.ParsedBody.(*types.AnthropicMessagesRequest)

	switch protocol.ExecutionMode(p.RouteMode) {
	case protocol.ModeNative:
		return h.adaptNative(c, req, p, logCapture)
	case protocol.ModeAdapter:
		switch protocol.AdapterKind(p.AdapterKind) {
		case protocol.AdapterMessagesToChat:
			return h.adaptToChat(c, req, p, logCapture)
		case protocol.AdapterMessagesToResponses:
			return h.adaptToResponses(c, req, p, logCapture)
		default:
			return nil, fmt.Errorf("unsupported adapter: %s", p.AdapterKind)
		}
	default:
		return nil, fmt.Errorf("unsupported route mode: %s", p.RouteMode)
	}
}

// --- Error handling ---

// HandlePlanError renders the protocol-specific error response for a Plan-phase
// rejection.  The RequestPlan's ErrorCode identifies the category.
func (h *Handler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	// Record preflight error if the span is still open.
	if pt := GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		SetPreflightTracer(c, nil)
	}

	// Record metrics for the error path so the error is still attributed
	// to the requested model.
	if h.MetricsRecorder != nil {
		var mt *types.ModelTarget
		if p != nil {
			mt = p.ModelTarget
		}
		h.MetricsRecorder.SetModelTarget(c, meta.Model, mt, meta.Streaming)
	}

	if p != nil {
		switch p.ErrorCode {
		case types.PlanErrModelNotFound:
			writeError(c, http.StatusNotFound, ErrTypeNotFound, err.Error())
			return
		case types.PlanErrModelUnavailable:
			writeError(c, http.StatusServiceUnavailable, ErrTypeOverloaded, err.Error())
			return
		case types.PlanErrInsufficientBalance:
			writeError(c, http.StatusPaymentRequired, ErrTypeInvalidRequest,
				fmt.Sprintf("insufficient balance: %v", err))
			return
		case types.PlanErrUsageLimitExceeded:
			writeError(c, http.StatusTooManyRequests, ErrTypeRateLimit,
				"usage quota exceeded for current window")
			return
		case types.PlanErrDisabled:
			writeError(c, http.StatusBadRequest, ErrTypeUnsupported,
				fmt.Sprintf("/v1/messages is not available for this model: %v", err))
			return
		case types.PlanErrInternal:
			writeError(c, http.StatusInternalServerError, ErrTypeAPI,
				"an internal error occurred while processing the request")
			return
		case types.PlanErrSensitive:
			message := "content blocked due to safety policy"
			if p.Safety != nil && p.Safety.Message != "" {
				message = p.Safety.Message
			}
			writeSensitiveResponse(c, meta.Streaming, &SensitiveResult{
				IsSensitive: true,
				Message:     message,
			})
			return
		}
	}
	slog.ErrorContext(c.Request.Context(), "messages plan error",
		slog.String("model", meta.Model), slog.Any("error", err))
	writeError(c, http.StatusInternalServerError, ErrTypeAPI, err.Error())
}

// extractUnsupportedCapabilities checks if the error is an unsupported
// capability error and returns the missing capabilities list.
func extractUnsupportedCapabilities(err error) []string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "unsupported_feature:") {
		return nil
	}
	feature := strings.TrimPrefix(msg, "unsupported_feature:")
	return []string{feature}
}

// writeSensitiveResponse writes a blocked-content response in Anthropic format.
func writeSensitiveResponse(c *gin.Context, stream bool, result *SensitiveResult) {
	message := "content blocked due to safety policy"
	if result != nil && result.Message != "" {
		message = result.Message
	}
	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		msgID := newMessagesResponseID()
		writeSSEEvent(c, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    msgID,
				"type":  "message",
				"role":  "assistant",
				"model": "",
				"usage": map[string]any{"input_tokens": 0, "output_tokens": 0},
			},
		})
		writeSSEEvent(c, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
		writeSSEEvent(c, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": message},
		})
		writeSSEEvent(c, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": 0,
		})
		stopReason := "end_turn"
		writeSSEEvent(c, "message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": 0},
		})
		writeSSEEvent(c, "message_stop", map[string]any{"type": "message_stop"})
		return
	}
	c.JSON(http.StatusOK, types.AnthropicMessagesResponse{
		ID:      newMessagesResponseID(),
		Type:    "message",
		Role:    "assistant",
		Model:   "",
		Content: []types.AnthropicContentBlock{{Type: "text", Text: message}},
		Usage:   types.AnthropicMessagesUsage{},
	})
}

// writeSSEEvent marshals data as JSON and writes it as an SSE event.
func writeSSEEvent(c *gin.Context, eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	c.Writer.Flush()
}
