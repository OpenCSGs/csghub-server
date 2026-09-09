package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// extractMessagesSessionID extracts the conversation/session ID from request
// headers for trace correlation.  Mirrors handler.extractChatSessionID.
func extractMessagesSessionID(headers http.Header) string {
	for _, header := range []string{"X-Claude-Code-Session-Id", "X-Session-ID", "X-Conversation-ID"} {
		if value := strings.TrimSpace(headers.Get(header)); value != "" {
			if len(value) > messagesSessionKeyMaxLen {
				return value[:messagesSessionKeyMaxLen]
			}
			return value
		}
	}
	return ""
}

const messagesSessionKeyMaxLen = 256

// messagesTraceTools converts Anthropic tools to GenerationToolDefinition
// for LLM trace.
func messagesTraceTools(req *types.AnthropicMessagesRequest) []types.GenerationToolDefinition {
	if req == nil || len(req.Tools) == 0 {
		return nil
	}
	tools := make([]types.GenerationToolDefinition, 0, len(req.Tools))
	for _, tool := range req.Tools {
		schema := tool.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		tools = append(tools, types.GenerationToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Type:        "function",
			InputSchema: schema,
		})
	}
	return tools
}

// messagesTraceMaxTokens returns the max_tokens from the request as *int64
// for LLM trace.
func messagesTraceMaxTokens(req *types.AnthropicMessagesRequest) *int64 {
	if req == nil || req.MaxTokens == 0 {
		return nil
	}
	value := int64(req.MaxTokens)
	return &value
}

// finishLLMTraceWithError records an error on the generation recorder and
// ends it.  Mirrors handler.finishLLMTraceWithError.
func finishLLMTraceWithError(recorder GenerationRecorder, err error, code string) {
	if recorder == nil || err == nil {
		return
	}
	recorder.SetError(err, code)
	recorder.End()
}

// recordMessagesTraceCompletion records the generation response and usage on
// the trace recorder.  Mirrors handler.recordChatTraceCompletion.
func recordMessagesTraceCompletion(input tracePostProcessInput, model *types.Model, targetModelName string, usage *tokenUsage, inputMsgs []types.GenerationMessage, outputMsgs []types.GenerationMessage, traceInfo commontypes.LLMLogTraceInfo) {
	if input.Recorder == nil {
		return
	}
	provider := ""
	if model != nil {
		provider = model.Provider
	}
	firstChunkAt := time.Time{}
	if input.Stream {
		firstChunkAt = input.FirstWriteAt
	}
	input.Recorder.SetFirstChunk(types.GenerationFirstChunk{At: firstChunkAt})
	input.Recorder.SetResponse(types.GenerationResponse{
		Provider:      provider,
		Model:         targetModelName,
		ResponseModel: targetModelName,
		Input:         inputMsgs,
		Output:        outputMsgs,
		ResponseID:    traceInfo.ResponseID,
		FinishReasons: traceInfo.FinishReasons,
	})
	if input.StatusCode >= http.StatusBadRequest {
		input.Recorder.SetError(httpStatusTraceError(input.StatusCode), types.TraceErrUpstreamError)
	}
	if usage != nil {
		input.Recorder.SetUsage(types.TokenUsage{
			InputTokens:           usage.PromptTokens,
			OutputTokens:          usage.CompletionTokens,
			TotalTokens:           usage.TotalTokens,
			CacheReadInputTokens:  usage.CachedPromptTokens,
			CacheWriteInputTokens: usage.CacheCreationPromptTokens,
		})
	}
}

func httpStatusTraceError(statusCode int) error {
	return fmt.Errorf("HTTP %d", statusCode)
}

// llmlogMessagesToGenerationMessages converts LLMLogMessage slices to
// GenerationMessage slices for trace.  Mirrors handler.llmlogMessagesToGenerationMessages.
func llmlogMessagesToGenerationMessages(msgs []commontypes.LLMLogMessage) []types.GenerationMessage {
	if len(msgs) == 0 {
		return nil
	}
	result := make([]types.GenerationMessage, 0, len(msgs))
	for _, msg := range msgs {
		gm := convertLLMLogMessage(msg)
		if len(gm.Parts) > 0 {
			result = append(result, gm)
		}
	}
	return result
}

func convertLLMLogMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	switch msg.Role {
	case "tool_call":
		return convertToolCallMessage(msg)
	case "tool_response":
		return types.GenerationMessage{
			Role: "tool",
			Parts: []types.GenerationPart{{
				Kind: "text",
				Text: msg.Content,
			}},
		}
	default:
		return convertRegularMessage(msg)
	}
}

func convertToolCallMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	var tc struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &tc); err != nil {
		return types.GenerationMessage{
			Role: "assistant",
			Parts: []types.GenerationPart{{
				Kind: "text",
				Text: msg.Content,
			}},
		}
	}
	return types.GenerationMessage{
		Role: "assistant",
		Parts: []types.GenerationPart{{
			Kind: "tool_call",
			ToolCall: &types.GenerationToolCall{
				Name:      tc.Name,
				InputJSON: llmlogToolCallInputJSON(tc.Arguments),
			},
		}},
	}
}

func llmlogToolCallInputJSON(arguments any) []byte {
	switch v := arguments.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, []byte(v)); err == nil {
			return compacted.Bytes()
		}
		return []byte(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return data
	}
}

func convertRegularMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	parts := make([]types.GenerationPart, 0, 2)
	if msg.ReasoningContent != "" {
		parts = append(parts, types.GenerationPart{
			Kind:     "thinking",
			Thinking: msg.ReasoningContent,
		})
	}
	if msg.Content != "" {
		parts = append(parts, types.GenerationPart{
			Kind: "text",
			Text: msg.Content,
		})
	}
	return types.GenerationMessage{
		Role:  msg.Role,
		Parts: parts,
	}
}

// isSuccessfulStatus returns true for HTTP 2xx status codes.
func isSuccessfulStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
