package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/handler/jev"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/proxy"
)

// jevHandlerBridge adapts OpenAIHandlerImpl to the jev.Handler dependency
// interfaces.  It delegates the upstream pass-through (the gateway's unified
// reverse proxy), usage recording, metrics recording, LLM tracing, and LLM
// log publishing to the existing handler infrastructure.
//
// The Planner is constructed directly from OpenAIHandlerImpl and does not
// go through the bridge.
type jevHandlerBridge struct {
	handler *OpenAIHandlerImpl
}

// newJevHandlerBridge creates a bridge from an OpenAIHandlerImpl.  The
// upstream exchange timeout comes from the AIGateway configuration; values
// <= 0 fall back to the component's built-in default.
func newJevHandlerBridge(h *OpenAIHandlerImpl) *jevHandlerBridge {
	return &jevHandlerBridge{handler: h}
}

// toJevDeps converts the bridge into the Deps struct expected by jev.New.
// The bridge implements all required interfaces.
func (b *jevHandlerBridge) toJevDeps() jev.Deps {
	deps := jev.Deps{
		ProxyExecutor:      b,
		UsageRecorder:      b,
		AdmissionFinalizer: b,
		MetricsRecorder:    b,
		ErrorRecorder:      errorRecorderFunc(SetMetricsError),
	}
	if b.handler.llmTracer != nil {
		deps.LLMTracer = &jevTracerAdapter{tracer: b.handler.llmTracer}
	}
	return deps
}

// --- ProxyExecutor ---

// ServeProxy forwards the request to the upstream through the gateway's
// unified reverse proxy.  The upstream path is taken from the resolved
// backend URL so the client's request path (/v1/systemone) is overridden
// with the correct upstream path.
func (b *jevHandlerBridge) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	rp, err := proxy.NewReverseProxy(backendURL, proxy.WithoutAcceptEncoding())
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadGateway)
		return
	}
	apiPath := ""
	if parsed, perr := url.Parse(backendURL); perr == nil && parsed.Path != "" {
		apiPath = parsed.Path
	}
	rp.ServeHTTP(responseWriter, c.Request, apiPath, host)
}

// --- UsageRecorder ---

func (b *jevHandlerBridge) RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, tokenID int64) error {
	return b.handler.openaiComponent.RecordUsageFromTokenUsage(ctx, nsUUID, model, targetModelName, usage, apikey, tokenID)
}

// --- AdmissionFinalizer ---

// FinalizeCapacityAdmission converts the jev-local usage into the gateway
// token usage and closes the capacity admission lease.
func (b *jevHandlerBridge) FinalizeCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *types.AdmissionUsage) {
	if usage == nil {
		b.handler.openaiComponent.FinalizeCapacityAdmission(ctx, lease, nil)
		return
	}
	b.handler.openaiComponent.FinalizeCapacityAdmission(ctx, lease, &token.Usage{
		TotalTokens: usage.TotalTokens,
	})
}

// --- MetricsRecorder ---

func (b *jevHandlerBridge) RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64) {
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

// --- ErrorRecorder ---

// errorRecorderFunc adapts the build-tag-gated SetMetricsError helper to the
// jev.ErrorRecorder interface.
type errorRecorderFunc func(c *gin.Context, errorType, errorMessage string)

func (f errorRecorderFunc) SetMetricsError(c *gin.Context, errorType, errorMessage string) {
	f(c, errorType, errorMessage)
}

// --- LLMTracer ---

// jevTracerAdapter wraps the handler's llmtrace.LLMTracer to satisfy
// jev.LLMTracer.  System One is synchronous, so only StartGeneration is
// used.
type jevTracerAdapter struct {
	tracer llmtrace.LLMTracer
}

func (a *jevTracerAdapter) StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, jev.GenerationRecorder) {
	traceCtx, recorder := a.tracer.StartGeneration(ctx, input)
	if traceCtx == nil {
		traceCtx = ctx
	}
	if recorder == nil {
		return traceCtx, nil
	}
	return traceCtx, &jevGenerationRecorderWrapper{recorder: recorder}
}

// jevGenerationRecorderWrapper adapts trace.GenerationRecorder to
// jev.GenerationRecorder.
type jevGenerationRecorderWrapper struct {
	recorder llmtrace.GenerationRecorder
}

func (w *jevGenerationRecorderWrapper) SetUsage(usage types.TokenUsage) {
	w.recorder.SetUsage(usage)
}

func (w *jevGenerationRecorderWrapper) SetResponse(response types.GenerationResponse) {
	w.recorder.SetResponse(response)
}

func (w *jevGenerationRecorderWrapper) SetFirstChunk(firstChunk types.GenerationFirstChunk) {
	w.recorder.SetFirstChunk(firstChunk)
}

func (w *jevGenerationRecorderWrapper) SetError(err error, code string) {
	w.recorder.SetError(err, code)
}

func (w *jevGenerationRecorderWrapper) End() {
	w.recorder.End()
}
