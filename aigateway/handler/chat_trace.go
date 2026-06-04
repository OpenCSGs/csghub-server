package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
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
	if chatReq.Stream {
		mode = types.GenerationModeStream
	}
	return h.startLLMTrace(ctx, types.GenerationStart{
		RequestID:      requestID,
		ConversationID: extractChatSessionID(headers),
		UserID:         userID,
		Provider:       modelTarget.Model.Provider,
		RequestModel:   modelID,
		ResolvedModel:  modelTarget.ModelName,
		Mode:           mode,
		Tools:          chatTraceTools(chatReq),
		ToolCount:      len(chatReq.Tools),
		MaxTokens:      chatTraceMaxTokens(chatReq),
		Temperature:    chatTraceTemperature(chatReq),
		TopP:           chatTraceTopP(chatReq),
		ToolChoice:     chatTraceToolChoice(chatReq),
		Metadata: map[string]any{
			llmtrace.TraceMetadataKeyAIGatewayAPI:     "/v1/chat/completions",
			llmtrace.TraceMetadataKeyAIGatewayModelID: modelTarget.Model.ID,
		},
	}, chatReq.Stream)
}

type chatTraceToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func chatTraceTools(chatReq *ChatCompletionRequest) []types.GenerationToolDefinition {
	if chatReq == nil || len(chatReq.Tools) == 0 {
		return nil
	}
	tools := make([]types.GenerationToolDefinition, 0, len(chatReq.Tools))
	for _, tool := range chatReq.Tools {
		data, err := json.Marshal(tool)
		if err != nil {
			continue
		}
		var parsed chatTraceToolDefinition
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		if parsed.Function.Name == "" {
			continue
		}
		toolType := parsed.Type
		if toolType == "" {
			toolType = "function"
		}
		tools = append(tools, types.GenerationToolDefinition{
			Name:        parsed.Function.Name,
			Description: parsed.Function.Description,
			Type:        toolType,
			InputSchema: parsed.Function.Parameters,
		})
	}
	return tools
}

func chatTraceToolChoice(chatReq *ChatCompletionRequest) *string {
	if chatReq == nil {
		return nil
	}
	data, err := json.Marshal(chatReq.ToolChoice)
	if err != nil {
		return nil
	}
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" || value == "{}" {
		return nil
	}
	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		if stringValue = strings.TrimSpace(stringValue); stringValue != "" {
			return &stringValue
		}
		return nil
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, data); err == nil {
		compactValue := compacted.String()
		return &compactValue
	}
	return &value
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

func recordChatTraceCompletion(input chatTracePostProcessInput, provider string, model string, usage *token.Usage, inputMsgs []types.GenerationMessage, outputMsgs []types.GenerationMessage, traceInfo commontypes.LLMLogTraceInfo) {
	if input.Recorder == nil {
		return
	}
	if input.Completion {
		firstChunkAt := time.Time{}
		if input.Stream {
			firstChunkAt = input.FirstWriteAt
		}
		recordLLMTraceCompletion(llmTraceCompletionInput{
			Recorder:      input.Recorder,
			Provider:      provider,
			Model:         model,
			Usage:         usage,
			Input:         inputMsgs,
			Output:        outputMsgs,
			ResponseID:    traceInfo.ResponseID,
			FinishReasons: traceInfo.FinishReasons,
			FirstChunkAt:  firstChunkAt,
			StatusCode:    input.StatusCode,
		})
		return
	}
	recordLLMTraceUsage(input.Recorder, usage)
}
