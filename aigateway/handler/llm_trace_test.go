package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestStartLLMTrace(t *testing.T) {
	t.Run("sync", func(t *testing.T) {
		recorder := &testGenerationRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		h := &OpenAIHandlerImpl{llmTracer: tracer}

		_, got := h.startLLMTrace(context.Background(), types.GenerationStart{RequestID: "req-sync"}, false)

		require.Same(t, recorder, got)
		starts := tracer.Starts()
		require.Len(t, starts, 1)
		require.Equal(t, "req-sync", starts[0].RequestID)
		require.Equal(t, []bool{false}, tracer.Streams())
	})

	t.Run("stream", func(t *testing.T) {
		recorder := &testGenerationRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		h := &OpenAIHandlerImpl{llmTracer: tracer}

		_, got := h.startLLMTrace(context.Background(), types.GenerationStart{RequestID: "req-stream"}, true)

		require.Same(t, recorder, got)
		starts := tracer.Starts()
		require.Len(t, starts, 1)
		require.Equal(t, "req-stream", starts[0].RequestID)
		require.Equal(t, []bool{true}, tracer.Streams())
	})

	t.Run("nil tracer", func(t *testing.T) {
		ctx := context.Background()
		h := &OpenAIHandlerImpl{}

		gotCtx, got := h.startLLMTrace(ctx, types.GenerationStart{RequestID: "req"}, false)

		require.Equal(t, ctx, gotCtx)
		require.Nil(t, got)
	})
}

func TestRecordLLMTraceCompletion(t *testing.T) {
	firstChunkAt := time.Now()
	recorder := &testGenerationRecorderWithMutex{}

	recordLLMTraceCompletion(llmTraceCompletionInput{
		Recorder:     recorder,
		Provider:     "openai",
		Model:        "gpt-test",
		Usage:        &token.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3, CachedPromptTokens: 4, CacheCreationPromptTokens: 5, ReasoningTokens: 6},
		FirstChunkAt: firstChunkAt,
		StatusCode:   http.StatusInternalServerError,
	})

	usage, response, errorCode, _, events := generationTraceSnapshot(recorder)
	require.NotNil(t, usage)
	require.Equal(t, int64(1), usage.InputTokens)
	require.Equal(t, int64(2), usage.OutputTokens)
	require.Equal(t, int64(3), usage.TotalTokens)
	require.Equal(t, int64(4), usage.CacheReadInputTokens)
	require.Equal(t, int64(5), usage.CacheWriteInputTokens)
	require.Equal(t, int64(6), usage.ReasoningTokens)
	require.NotNil(t, response)
	require.Equal(t, "openai", response.Provider)
	require.Equal(t, "gpt-test", response.Model)
	require.Equal(t, types.TraceErrUpstreamError, errorCode)
	require.Equal(t, []string{"first_chunk", "response", "error", "usage"}, events)
}

func TestFinishLLMTraceWithError(t *testing.T) {
	recorder := &testGenerationRecorderWithMutex{}

	finishLLMTraceWithError(recorder, errors.New("failed"), types.TraceErrUpstreamUnavailable)

	_, _, errorCode, ended, events := generationTraceSnapshot(recorder)
	require.True(t, ended)
	require.Equal(t, types.TraceErrUpstreamUnavailable, errorCode)
	require.Equal(t, []string{"error", "end"}, events)
}

func generationTraceSnapshot(recorder *testGenerationRecorderWithMutex) (*types.TokenUsage, *types.GenerationResponse, string, bool, []string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	events := append([]string(nil), recorder.events...)
	return recorder.usage, recorder.response, recorder.errorCode, recorder.ended, events
}
