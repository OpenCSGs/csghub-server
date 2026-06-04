package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type llmTraceCompletionInput struct {
	Recorder      llmtrace.GenerationRecorder
	Provider      string
	Model         string
	Usage         *token.Usage
	Input         []types.GenerationMessage
	Output        []types.GenerationMessage
	ResponseID    string
	FinishReasons []string
	FirstChunkAt  time.Time
	StatusCode    int
}

func (h *OpenAIHandlerImpl) startLLMTrace(ctx context.Context, input types.GenerationStart, stream bool) (context.Context, llmtrace.GenerationRecorder) {
	if h == nil || h.llmTracer == nil {
		return ctx, nil
	}
	start := h.llmTracer.StartGeneration
	if stream {
		start = h.llmTracer.StartStreamingGeneration
	}
	traceCtx, recorder := start(ctx, input)
	if traceCtx == nil {
		traceCtx = ctx
	}
	return traceCtx, recorder
}

func recordLLMTraceCompletion(input llmTraceCompletionInput) {
	if input.Recorder == nil {
		return
	}
	if !input.FirstChunkAt.IsZero() {
		input.Recorder.SetFirstChunk(types.GenerationFirstChunk{
			At: input.FirstChunkAt,
		})
	}
	input.Recorder.SetResponse(types.GenerationResponse{
		Provider:      input.Provider,
		Model:         input.Model,
		ResponseModel: input.Model,
		Input:         input.Input,
		Output:        input.Output,
		ResponseID:    input.ResponseID,
		FinishReasons: input.FinishReasons,
	})
	if input.StatusCode >= http.StatusBadRequest {
		input.Recorder.SetError(httpStatusTraceError(input.StatusCode), types.TraceErrUpstreamError)
	}
	recordLLMTraceUsage(input.Recorder, input.Usage)
}

func recordLLMTraceUsage(recorder llmtrace.GenerationRecorder, usage *token.Usage) {
	if recorder == nil || usage == nil {
		return
	}
	recorder.SetUsage(types.TokenUsage{
		InputTokens:           usage.PromptTokens,
		OutputTokens:          usage.CompletionTokens,
		TotalTokens:           usage.TotalTokens,
		CacheReadInputTokens:  usage.CachedPromptTokens,
		CacheWriteInputTokens: usage.CacheCreationPromptTokens,
		ReasoningTokens:       usage.ReasoningTokens,
	})
}

func finishLLMTraceWithError(recorder llmtrace.GenerationRecorder, err error, code string) {
	if recorder == nil || err == nil {
		return
	}
	recorder.SetError(err, code)
	recorder.End()
}

func httpStatusTraceError(statusCode int) error {
	return fmt.Errorf("HTTP %d", statusCode)
}
