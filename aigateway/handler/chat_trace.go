package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

const (
	sessionHeaderClaudeCode = "X-Claude-Code-Session-Id"
	sessionHeaderSessionID  = "X-Session-ID"
	sessionHeaderConvID     = "X-Conversation-ID"
)

func extractChatSessionID(headers http.Header) string {
	for _, header := range []string{sessionHeaderClaudeCode, sessionHeaderSessionID, sessionHeaderConvID} {
		if value := strings.TrimSpace(headers.Get(header)); value != "" {
			if len(value) > maxSessionKeyLength {
				return value[:maxSessionKeyLength]
			}
			return value
		}
	}
	return ""
}

func (h *OpenAIHandlerImpl) startChatTrace(ctx context.Context, headers http.Header, modelID string, modelTarget *resolvedModelTarget, chatReq *ChatCompletionRequest, requestID string, userID string) (context.Context, llmtrace.GenerationRecorder) {
	if h == nil || h.llmTracer == nil || modelTarget == nil || modelTarget.Model == nil || chatReq == nil {
		return ctx, nil
	}
	mode := types.GenerationModeSync
	start := h.llmTracer.StartGeneration
	if chatReq.Stream {
		mode = types.GenerationModeStream
		start = h.llmTracer.StartStreamingGeneration
	}
	traceCtx, recorder := start(ctx, types.GenerationStart{
		RequestID:      requestID,
		ConversationID: extractChatSessionID(headers),
		UserID:         userID,
		Provider:       modelTarget.Model.Provider,
		RequestModel:   modelID,
		ResolvedModel:  modelTarget.ModelName,
		Mode:           mode,
		ToolCount:      len(chatReq.Tools),
		MaxTokens:      chatTraceMaxTokens(chatReq),
		Temperature:    chatTraceTemperature(chatReq),
		TopP:           chatTraceTopP(chatReq),
		Metadata: map[string]any{
			"aigateway.model.id": modelTarget.Model.ID,
		},
	})
	if traceCtx == nil {
		traceCtx = ctx
	}
	return traceCtx, recorder
}

func chatTraceMaxTokens(chatReq *ChatCompletionRequest) *int64 {
	if chatReq == nil || chatReq.MaxTokens == 0 {
		return nil
	}
	value := int64(chatReq.MaxTokens)
	return &value
}

func chatTraceTemperature(chatReq *ChatCompletionRequest) *float64 {
	if chatReq == nil || chatReq.Temperature == 0 {
		return nil
	}
	value := chatReq.Temperature
	return &value
}

func chatTraceTopP(chatReq *ChatCompletionRequest) *float64 {
	if chatReq == nil || chatReq.TopP == 0 {
		return nil
	}
	value := chatReq.TopP
	return &value
}

func recordChatTraceUsage(recorder llmtrace.GenerationRecorder, usage *token.Usage) {
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

type chatTracePostProcessInput struct {
	Recorder     llmtrace.GenerationRecorder
	Completion   bool
	Stream       bool
	FirstWriteAt time.Time
	StatusCode   int
}

func newChatTracePostProcessInput(recorder llmtrace.GenerationRecorder, chatReq *ChatCompletionRequest, writer *chatRetryResponseWriter) chatTracePostProcessInput {
	input := chatTracePostProcessInput{
		Recorder:   recorder,
		Completion: recorder != nil,
	}
	if chatReq != nil {
		input.Stream = chatReq.Stream
	}
	if writer != nil {
		input.FirstWriteAt = writer.FirstWriteAt()
		input.StatusCode = writer.StatusCode()
	}
	return input
}

func recordChatTraceCompletion(input chatTracePostProcessInput, provider string, model string, usage *token.Usage) {
	if input.Recorder == nil {
		return
	}
	if input.Completion {
		recordChatTraceFirstChunk(input.Recorder, input.Stream, input.FirstWriteAt)
		recordChatTraceResponse(input.Recorder, provider, model)
		recordChatTraceHTTPStatus(input.Recorder, input.StatusCode)
	}
	recordChatTraceUsage(input.Recorder, usage)
}

func recordChatTraceFirstChunk(recorder llmtrace.GenerationRecorder, stream bool, firstWriteAt time.Time) {
	if recorder == nil || !stream || firstWriteAt.IsZero() {
		return
	}
	recorder.SetFirstChunk(types.GenerationFirstChunk{
		At: firstWriteAt,
	})
}

func recordChatTraceResponse(recorder llmtrace.GenerationRecorder, provider string, model string) {
	if recorder == nil {
		return
	}
	response := types.GenerationResponse{
		Provider:      provider,
		Model:         model,
		ResponseModel: model,
	}
	recorder.SetResponse(response)
}

func recordChatTraceHTTPStatus(recorder llmtrace.GenerationRecorder, statusCode int) {
	if recorder == nil {
		return
	}
	if statusCode < http.StatusBadRequest {
		return
	}
	recorder.SetError(fmt.Errorf("HTTP %d", statusCode), types.TraceErrUpstreamError)
}

func finishChatTraceWithError(recorder llmtrace.GenerationRecorder, err error, code string) {
	if recorder == nil || err == nil {
		return
	}
	recorder.SetError(err, code)
	recorder.End()
}
