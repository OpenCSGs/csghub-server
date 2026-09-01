package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnthropicMessagesRequest represents a POST /v1/messages request
// following the Anthropic Messages API specification.
// Unknown fields are preserved in ExtraFields for native passthrough.
type AnthropicMessagesRequest struct {
	Model          string                 `json:"model"`
	Messages       []AnthropicMessage     `json:"messages"`
	System         json.RawMessage        `json:"system,omitempty"` // string or []AnthropicContentBlock
	MaxTokens      int                    `json:"max_tokens"`
	Metadata       *AnthropicMetadata     `json:"metadata,omitempty"`
	StopSequences  []string               `json:"stop_sequences,omitempty"`
	Stream         bool                   `json:"stream,omitempty"`
	Temperature    *float64               `json:"temperature,omitempty"`
	TopP           *float64               `json:"top_p,omitempty"`
	TopK           *int                   `json:"top_k,omitempty"`
	Tools          []AnthropicTool        `json:"tools,omitempty"`
	ToolChoice     json.RawMessage        `json:"tool_choice,omitempty"`
	Thinking       *AnthropicThinking     `json:"thinking,omitempty"`
	ExtraFields    map[string]json.RawMessage `json:"-"`
}

// AnthropicMessage is a single message in the messages array.
type AnthropicMessage struct {
	Role    string          `json:"role"` // "user" | "assistant"
	Content json.RawMessage `json:"content"` // string or []AnthropicContentBlock
}

// AnthropicContentBlock is a typed content block within a message.
type AnthropicContentBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Source       *AnthropicImageSource  `json:"source,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Input        json.RawMessage        `json:"input,omitempty"`
	ToolUseID    string                 `json:"tool_use_id,omitempty"`
	Content      json.RawMessage        `json:"content,omitempty"` // for tool_result: string or []block
	Thinking     string                 `json:"thinking,omitempty"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

// AnthropicImageSource represents an image source in an image content block.
type AnthropicImageSource struct {
	Type      string `json:"type"`       // "base64" | "url"
	MediaType string `json:"media_type"` // "image/png", "image/jpeg", etc.
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// AnthropicCacheControl enables prompt caching for a content block.
type AnthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// AnthropicMetadata is optional user-provided metadata.
type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicThinking configures extended thinking.
type AnthropicThinking struct {
	Type         string `json:"type"` // "enabled" | "disabled" | "adaptive"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// AnthropicTool describes a tool the model may call.
// It supports both the native Anthropic format (name / description / input_schema)
// and the OpenAI Chat Completions format (type: "function", function: {name, description, parameters}).
type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

// UnmarshalJSON handles both Anthropic and OpenAI Chat tool formats.
func (t *AnthropicTool) UnmarshalJSON(data []byte) error {
	// First, try the native Anthropic format.
	type raw AnthropicTool
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	// If the native format has a name, use it directly.
	if r.Name != "" {
		*t = AnthropicTool(r)
		return nil
	}
	// If name is empty, the input may be in OpenAI Chat format:
	//   {"type": "function", "function": {"name": "...", "description": "...", "parameters": {...}}}
	var chatTool struct {
		Type     string `json:"type"`
		Function struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(data, &chatTool); err != nil {
		return err
	}
	if chatTool.Type == "function" && chatTool.Function.Name != "" {
		t.Name = chatTool.Function.Name
		t.Description = chatTool.Function.Description
		t.InputSchema = chatTool.Function.Parameters
		return nil
	}
	// Fall back to the native parsed result (may have empty name).
	*t = AnthropicTool(r)
	return nil
}

// Validate checks required fields in a Messages request.
func (r *AnthropicMessagesRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if r.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens is required and must be positive")
	}
	if len(r.Messages) == 0 {
		return fmt.Errorf("messages is required and must not be empty")
	}
	return nil
}

// UnmarshalJSON preserves unknown fields in ExtraFields.
func (r *AnthropicMessagesRequest) UnmarshalJSON(data []byte) error {
	type alias AnthropicMessagesRequest
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	for _, key := range []string{
		"model", "messages", "system", "max_tokens", "metadata",
		"stop_sequences", "stream", "temperature", "top_p", "top_k",
		"tools", "tool_choice", "thinking",
	} {
		delete(allFields, key)
	}
	tmp.ExtraFields = allFields
	*r = AnthropicMessagesRequest(tmp)
	return nil
}

// MarshalJSON merges ExtraFields back into the output.
func (r AnthropicMessagesRequest) MarshalJSON() ([]byte, error) {
	type alias AnthropicMessagesRequest
	known, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.ExtraFields) == 0 {
		return known, nil
	}
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(known, &knownFields); err != nil {
		return nil, err
	}
	for k, v := range r.ExtraFields {
		if _, exists := knownFields[k]; !exists {
			knownFields[k] = v
		}
	}
	return json.Marshal(knownFields)
}

// AnthropicMessagesResponse is the non-streaming response for POST /v1/messages.
type AnthropicMessagesResponse struct {
	ID           string                  `json:"id"` // "msg_xxx"
	Type         string                  `json:"type"` // "message"
	Role         string                  `json:"role"` // "assistant"
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   *string                 `json:"stop_reason,omitempty"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        AnthropicMessagesUsage  `json:"usage"`
}

// AnthropicMessagesUsage reports token usage.
type AnthropicMessagesUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// AnthropicMessagesError is the error response shape for the Anthropic API.
type AnthropicMessagesError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// --- Stream event types ---

// AnthropicStreamEvent is a single SSE event in a streaming Messages response.
// The Type field determines which sub-fields are populated.
type AnthropicStreamEvent struct {
	Type         string                  `json:"type"`
	Message      *AnthropicStreamMessage `json:"message,omitempty"`
	Index        *int                    `json:"index,omitempty"`
	ContentBlock *AnthropicContentBlock  `json:"content_block,omitempty"`
	Delta        json.RawMessage         `json:"delta,omitempty"`
	Usage        *AnthropicMessagesUsage `json:"usage,omitempty"`
}

// AnthropicStreamMessage is the message object in a message_start event.
type AnthropicStreamMessage struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content,omitempty"`
	Model        string                  `json:"model"`
	StopReason   *string                 `json:"stop_reason,omitempty"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        AnthropicMessagesUsage  `json:"usage"`
}

// --- Helper functions ---

// AnthropicMessageContentText extracts the text content from a message's content field.
// Handles both string content and []ContentBlock content.
func AnthropicMessageContentText(raw json.RawMessage) string {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					parts = append(parts, b.Text)
				}
			case "tool_result":
				// tool_result content is user-supplied and should be checked
				// by moderation.  It can be a string or an array of text blocks.
				if text := AnthropicMessageContentText(b.Content); text != "" {
					parts = append(parts, text)
				}
			case "tool_use":
				// tool_use input is model-generated but may contain user-influenced
				// content; include for moderation completeness.
				if len(b.Input) > 0 && string(b.Input) != "null" {
					parts = append(parts, string(b.Input))
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// ParseAnthropicContentBlocks parses a message content field (string or []block)
// into a slice of AnthropicContentBlock.  A plain string becomes a single
// text block.
func ParseAnthropicContentBlocks(raw json.RawMessage) ([]AnthropicContentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return []AnthropicContentBlock{{Type: "text", Text: asString}}, nil
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, fmt.Errorf("content must be a string or array of content blocks: %w", err)
	}
	return blocks, nil
}

// HasAnthropicCacheControl checks whether any content block in the request
// carries a cache_control directive.
func (r *AnthropicMessagesRequest) HasCacheControl() bool {
	for _, msg := range r.Messages {
		blocks, err := ParseAnthropicContentBlocks(msg.Content)
		if err != nil {
			continue
		}
		for _, b := range blocks {
			if b.CacheControl != nil {
				return true
			}
		}
	}
	for _, tool := range r.Tools {
		if tool.CacheControl != nil {
			return true
		}
	}
	return false
}

// HasAnthropicVision checks whether any message contains an image content block.
func (r *AnthropicMessagesRequest) HasVision() bool {
	for _, msg := range r.Messages {
		blocks, err := ParseAnthropicContentBlocks(msg.Content)
		if err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == "image" {
				return true
			}
		}
	}
	return false
}

// HasAnthropicThinking checks whether the request enables extended thinking.
func (r *AnthropicMessagesRequest) HasThinking() bool {
	return r.Thinking != nil && (r.Thinking.Type == "enabled" || r.Thinking.Type == "adaptive")
}

// PromptText extracts a plain-text representation of the prompt for moderation.
func (r *AnthropicMessagesRequest) PromptText() string {
	var parts []string
	// System prompt
	if len(r.System) > 0 {
		if text := AnthropicMessageContentText(r.System); text != "" {
			parts = append(parts, text)
		}
	}
	// Messages
	for _, msg := range r.Messages {
		if text := AnthropicMessageContentText(msg.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}
