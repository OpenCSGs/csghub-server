package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/anthropic"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/proxy"
)

// messagesHandlerBridge adapts OpenAIHandlerImpl to the anthropic.Handler
// dependency interfaces.  It delegates proxy execution, usage recording,
// metrics recording, LLM tracing, and LLM log publishing to the existing
// handler infrastructure.
//
// The Planner is constructed directly from OpenAIHandlerImpl and does not
// go through the bridge.
type messagesHandlerBridge struct {
	handler *OpenAIHandlerImpl
}

// newMessagesHandlerBridge creates a bridge from an OpenAIHandlerImpl.
func newMessagesHandlerBridge(h *OpenAIHandlerImpl) *messagesHandlerBridge {
	return &messagesHandlerBridge{handler: h}
}

// toMessagesDeps converts the bridge into the Deps struct expected by
// anthropic.New.  The bridge implements all required interfaces.
func (b *messagesHandlerBridge) toMessagesDeps() anthropic.Deps {
	deps := anthropic.Deps{
		ProxyExecutor:   b,
		UsageRecorder:   b,
		UsageLimiter:    b,
		MetricsRecorder: b,
	}
	if b.handler.llmTracer != nil {
		deps.LLMTracer = &llmTracerAdapter{tracer: b.handler.llmTracer}
	}
	deps.LLMLogRecorderFactory = &llmLogRecorderFactoryAdapter{}
	if b.handler.llmLogPublisher != nil {
		deps.LLMLogPublisher = &llmLogPublisherAdapter{publisher: b.handler.llmLogPublisher}
	}
	return deps
}

// --- ProxyExecutor ---

func (b *messagesHandlerBridge) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	rp, err := proxy.NewReverseProxy(backendURL, proxy.WithoutAcceptEncoding())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": "failed to create reverse proxy: " + err.Error(),
			},
		})
		return
	}
	// Extract the path from backendURL so the reverse proxy director
	// overrides the client's request path (e.g. /v1/messages) with the
	// correct upstream path (e.g. /v1/chat/completions).
	apiPath := ""
	if parsed, perr := url.Parse(backendURL); perr == nil && parsed.Path != "" {
		apiPath = parsed.Path
	}
	rp.ServeHTTP(responseWriter, c.Request, apiPath, host)
}

// --- UsageRecorder ---

func (b *messagesHandlerBridge) RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64, apikey string) error {
	usage := &token.Usage{
		PromptTokens:              inputTokens,
		CompletionTokens:          outputTokens,
		TotalTokens:               inputTokens + outputTokens,
		CachedPromptTokens:        cachedPromptTokens,
		CacheCreationPromptTokens: cacheCreationPromptTokens,
	}
	return b.handler.openaiComponent.RecordUsageFromTokenUsage(ctx, nsUUID, model, targetModelName, usage, apikey)
}

// --- UsageLimiter (commit only) ---

func (b *messagesHandlerBridge) CommitUsageLimitFromUsage(ctx context.Context, nsUUID string, model *types.Model, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64) error {
	usage := &token.Usage{
		PromptTokens:              inputTokens,
		CompletionTokens:          outputTokens,
		TotalTokens:               inputTokens + outputTokens,
		CachedPromptTokens:        cachedPromptTokens,
		CacheCreationPromptTokens: cacheCreationPromptTokens,
	}
	return b.handler.openaiComponent.CommitUsageLimitFromUsage(ctx, nsUUID, model, usage)
}

// --- MetricsRecorder ---

func (b *messagesHandlerBridge) SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool) {
	var rt *resolvedModelTarget
	if target != nil {
		rt = &resolvedModelTarget{
			Model:     target.Model,
			Upstream:  target.Upstream,
			Target:    target.Target,
			Host:      target.Host,
			ModelName: target.ModelName,
		}
	}
	SetMetricsModelTarget(SetMetricsModelParams{
		C:           c,
		ModelID:     modelID,
		ModelTarget: rt,
		IsStream:    isStream,
	})
}

func (b *messagesHandlerBridge) RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64) {
	if inputTokens == 0 && outputTokens == 0 {
		return
	}
	usage := &token.Usage{
		PromptTokens:       inputTokens,
		CompletionTokens:   outputTokens,
		TotalTokens:        inputTokens + outputTokens,
		CachedPromptTokens: cachedPromptTokens,
	}
	RecordMetrics(RecordMetricsParams{
		C:     c,
		Ctx:   c.Request.Context(),
		Usage: usage,
	})
}

// --- LLMTracer ---

// llmTracerAdapter wraps the handler's llmtrace.LLMTracer to satisfy
// anthropic.LLMTracer.  It selects StartGeneration or StartStreamingGeneration
// based on the GenerationStart.Mode.
type llmTracerAdapter struct {
	tracer llmtrace.LLMTracer
}

func (a *llmTracerAdapter) StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, anthropic.GenerationRecorder) {
	var start func(ctx context.Context, input types.GenerationStart) (context.Context, llmtrace.GenerationRecorder)
	if input.Mode == types.GenerationModeStream {
		start = a.tracer.StartStreamingGeneration
	} else {
		start = a.tracer.StartGeneration
	}
	traceCtx, recorder := start(ctx, input)
	if traceCtx == nil {
		traceCtx = ctx
	}
	if recorder == nil {
		return traceCtx, nil
	}
	return traceCtx, &generationRecorderWrapper{recorder: recorder}
}

// generationRecorderWrapper adapts trace.GenerationRecorder to
// anthropic.GenerationRecorder.
type generationRecorderWrapper struct {
	recorder llmtrace.GenerationRecorder
}

func (w *generationRecorderWrapper) SetUsage(usage types.TokenUsage) {
	w.recorder.SetUsage(usage)
}

func (w *generationRecorderWrapper) SetResponse(response types.GenerationResponse) {
	w.recorder.SetResponse(response)
}

func (w *generationRecorderWrapper) SetFirstChunk(firstChunk types.GenerationFirstChunk) {
	w.recorder.SetFirstChunk(firstChunk)
}

func (w *generationRecorderWrapper) SetError(err error, code string) {
	w.recorder.SetError(err, code)
}

func (w *generationRecorderWrapper) End() {
	w.recorder.End()
}

// --- LLMLogRecorderFactory ---

// llmLogRecorderFactoryAdapter wraps component.NewLLMLogRecorder to satisfy
// anthropic.LLMLogRecorderFactory.
type llmLogRecorderFactoryAdapter struct{}

func (a *llmLogRecorderFactoryAdapter) NewLLMLogRecorder(requestID, modelID, userUUID string, req commontypes.LLMLogRequest, metadata map[string]any) (anthropic.LLMLogRecorder, error) {
	recorder, err := component.NewLLMLogRecorder(requestID, modelID, userUUID, req, metadata)
	if err != nil {
		return nil, err
	}
	return &llmLogRecorderWrapper{recorder: recorder}, nil
}

// llmLogRecorderWrapper adapts component.LLMLogRecorder to
// anthropic.LLMLogRecorder.
type llmLogRecorderWrapper struct {
	recorder component.LLMLogRecorder
}

func (w *llmLogRecorderWrapper) Completion(resp types.ChatCompletion) {
	w.recorder.Completion(resp)
}

func (w *llmLogRecorderWrapper) AppendCompletionChunk(chunk types.ChatCompletionChunk) {
	w.recorder.AppendCompletionChunk(chunk)
}

func (w *llmLogRecorderWrapper) Record() (*commontypes.LLMLogRecord, error) {
	return w.recorder.Record()
}

func (w *llmLogRecorderWrapper) Messages() ([]commontypes.LLMLogMessage, []commontypes.LLMLogMessage) {
	return w.recorder.Messages()
}

func (w *llmLogRecorderWrapper) TraceInfo() commontypes.LLMLogTraceInfo {
	return w.recorder.TraceInfo()
}

// --- LLMLogPublisher ---

// llmLogPublisherAdapter wraps component.LLMLogPublisher to satisfy
// anthropic.LLMLogPublisher.
type llmLogPublisherAdapter struct {
	publisher component.LLMLogPublisher
}

func (a *llmLogPublisherAdapter) PublishTrainingLog(message []byte) error {
	return a.publisher.PublishTrainingLog(message)
}

// --- PreflightTracer ---

// preflightTracerAdapter wraps the handler's *preflightTrace to satisfy
// anthropic.PreflightTracer.  It converts *types.ModelTarget back to
// *resolvedModelTarget for SetTargetModel.
type preflightTracerAdapter struct {
	trace *preflightTrace
}

func (a *preflightTracerAdapter) RecordError(err error, errorType string) {
	a.trace.RecordError(err, errorType)
}

func (a *preflightTracerAdapter) SetTargetModel(requestModel string, target *types.ModelTarget) {
	var rt *resolvedModelTarget
	if target != nil {
		rt = &resolvedModelTarget{
			Model:     target.Model,
			Upstream:  target.Upstream,
			Target:    target.Target,
			Host:      target.Host,
			ModelName: target.ModelName,
		}
	}
	a.trace.SetTargetModel(requestModel, rt)
}

func (a *preflightTracerAdapter) End() {
	a.trace.End()
}
