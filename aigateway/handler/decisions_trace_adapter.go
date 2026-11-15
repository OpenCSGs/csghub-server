package handler

import (
	"context"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/handler/decisions"
	"opencsg.com/csghub-server/aigateway/types"
)

// decisionsTracerAdapter gives the protocol handler the same generation
// recorder used by the other synchronous inference handlers.
type decisionsTracerAdapter struct {
	tracer llmtrace.LLMTracer
}

func (a *decisionsTracerAdapter) StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, decisions.GenerationRecorder) {
	traceCtx, recorder := a.tracer.StartGeneration(ctx, input)
	if traceCtx == nil {
		traceCtx = ctx
	}
	if recorder == nil {
		return traceCtx, nil
	}
	return traceCtx, decisionsGenerationRecorderWrapper{recorder: recorder}
}

type decisionsGenerationRecorderWrapper struct {
	recorder llmtrace.GenerationRecorder
}

func (w decisionsGenerationRecorderWrapper) SetUsage(usage types.TokenUsage) {
	w.recorder.SetUsage(usage)
}

func (w decisionsGenerationRecorderWrapper) SetResponse(response types.GenerationResponse) {
	w.recorder.SetResponse(response)
}

func (w decisionsGenerationRecorderWrapper) SetFirstChunk(firstChunk types.GenerationFirstChunk) {
	w.recorder.SetFirstChunk(firstChunk)
}

func (w decisionsGenerationRecorderWrapper) SetError(err error, code string) {
	w.recorder.SetError(err, code)
}

func (w decisionsGenerationRecorderWrapper) End() {
	w.recorder.End()
}
