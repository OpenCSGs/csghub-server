package types

import (
	"encoding/json"

	"github.com/openai/openai-go/v3"
)

type LLMLogUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type LLMLogMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        json.RawMessage `json:"tool_calls,omitempty"`
	FinishReason     string          `json:"finish_reason,omitempty"`
}

type LLMLogRecord struct {
	RequestID  string          `json:"request_id"`
	EventTime  string          `json:"event_time"`
	SampleType string          `json:"sample_type"`
	ModelID    string          `json:"model_id"`
	UserUUID   string          `json:"user_uuid"`
	Tools      json.RawMessage `json:"tools"`
	Messages   []LLMLogMessage `json:"messages"`
	Usage      LLMLogUsage     `json:"usage"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
}

type LLMLogRequest struct {
	Messages []openai.ChatCompletionMessageParamUnion
	Tools    []openai.ChatCompletionToolUnionParam
	Stream   bool
}
