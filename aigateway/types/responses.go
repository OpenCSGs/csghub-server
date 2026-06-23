package types

import (
	"encoding/json"
	"fmt"
)

// ResponsesRequest is the AIGateway-owned DTO for OpenAI-compatible
// POST /v1/responses. Unknown fields are preserved for native passthrough.
type ResponsesRequest struct {
	Model                string                     `json:"model"`
	Input                json.RawMessage            `json:"input,omitempty"`
	Instructions         json.RawMessage            `json:"instructions,omitempty"`
	PreviousResponseID   string                     `json:"previous_response_id,omitempty"`
	Store                *bool                      `json:"store,omitempty"`
	Stream               bool                       `json:"stream,omitempty"`
	StreamOptions        json.RawMessage            `json:"stream_options,omitempty"`
	MaxOutputTokens      *int                       `json:"max_output_tokens,omitempty"`
	Temperature          *float64                   `json:"temperature,omitempty"`
	TopP                 *float64                   `json:"top_p,omitempty"`
	TopLogprobs          *int                       `json:"top_logprobs,omitempty"`
	Text                 json.RawMessage            `json:"text,omitempty"`
	Tools                json.RawMessage            `json:"tools,omitempty"`
	ToolChoice           json.RawMessage            `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool                      `json:"parallel_tool_calls,omitempty"`
	Metadata             json.RawMessage            `json:"metadata,omitempty"`
	Background           *bool                      `json:"background,omitempty"`
	Conversation         json.RawMessage            `json:"conversation,omitempty"`
	Prompt               json.RawMessage            `json:"prompt,omitempty"`
	MaxToolCalls         *int                       `json:"max_tool_calls,omitempty"`
	Reasoning            json.RawMessage            `json:"reasoning,omitempty"`
	Include              json.RawMessage            `json:"include,omitempty"`
	Truncation           json.RawMessage            `json:"truncation,omitempty"`
	User                 string                     `json:"user,omitempty"`
	ServiceTier          string                     `json:"service_tier,omitempty"`
	ContextManagement    json.RawMessage            `json:"context_management,omitempty"`
	PromptCacheKey       string                     `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                     `json:"prompt_cache_retention,omitempty"`
	SafetyIdentifier     string                     `json:"safety_identifier,omitempty"`
	ExtraFields          map[string]json.RawMessage `json:"-"`
}

func (r *ResponsesRequest) UnmarshalJSON(data []byte) error {
	type alias ResponsesRequest
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	for _, key := range []string{
		"model", "input", "instructions", "previous_response_id", "store",
		"stream", "stream_options", "max_output_tokens", "temperature", "top_p", "top_logprobs", "text",
		"tools", "tool_choice", "parallel_tool_calls", "metadata", "background", "conversation",
		"prompt", "max_tool_calls", "reasoning", "include", "truncation", "user", "service_tier",
		"context_management", "prompt_cache_key", "prompt_cache_retention", "safety_identifier",
	} {
		delete(allFields, key)
	}
	tmp.ExtraFields = allFields
	*r = ResponsesRequest(tmp)
	return nil
}

func (r ResponsesRequest) MarshalJSON() ([]byte, error) {
	type alias ResponsesRequest
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

func (r ResponsesRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if (len(r.Input) == 0 || string(r.Input) == "null") && r.PreviousResponseID == "" {
		return fmt.Errorf("input is required")
	}
	return nil
}

type ResponsesUsage struct {
	InputTokens         int64                        `json:"input_tokens,omitempty"`
	OutputTokens        int64                        `json:"output_tokens,omitempty"`
	TotalTokens         int64                        `json:"total_tokens,omitempty"`
	InputTokensDetails  *ResponsesInputTokenDetails  `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ResponsesOutputTokenDetails `json:"output_tokens_details,omitempty"`
}

type ResponsesInputTokenDetails struct {
	CachedTokens         int64 `json:"cached_tokens,omitempty"`
	CachedCreationTokens int64 `json:"cached_creation_tokens,omitempty"`
	TextTokens           int64 `json:"text_tokens,omitempty"`
	AudioTokens          int64 `json:"audio_tokens,omitempty"`
	ImageTokens          int64 `json:"image_tokens,omitempty"`
}

type ResponsesOutputTokenDetails struct {
	TextTokens      int64 `json:"text_tokens,omitempty"`
	AudioTokens     int64 `json:"audio_tokens,omitempty"`
	ImageTokens     int64 `json:"image_tokens,omitempty"`
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

type ResponsesResponse struct {
	ID                 string                `json:"id"`
	Object             string                `json:"object"`
	CreatedAt          int64                 `json:"created_at"`
	Status             string                `json:"status"`
	IncompleteDetails  json.RawMessage       `json:"incomplete_details,omitempty"`
	Instructions       json.RawMessage       `json:"instructions,omitempty"`
	MaxOutputTokens    int                   `json:"max_output_tokens,omitempty"`
	Model              string                `json:"model"`
	Output             []ResponsesOutputItem `json:"output,omitempty"`
	OutputText         string                `json:"output_text,omitempty"`
	ParallelToolCalls  *bool                 `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
	Reasoning          json.RawMessage       `json:"reasoning,omitempty"`
	Store              *bool                 `json:"store,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	ToolChoice         json.RawMessage       `json:"tool_choice,omitempty"`
	Tools              json.RawMessage       `json:"tools,omitempty"`
	TopP               *float64              `json:"top_p,omitempty"`
	Truncation         json.RawMessage       `json:"truncation,omitempty"`
	User               string                `json:"user,omitempty"`
	Metadata           json.RawMessage       `json:"metadata,omitempty"`
	Usage              *ResponsesUsage       `json:"usage,omitempty"`
	Error              any                   `json:"error,omitempty"`
}

type ResponsesOutputItem struct {
	ID        string                 `json:"id,omitempty"`
	Type      string                 `json:"type"`
	Status    string                 `json:"status,omitempty"`
	Role      string                 `json:"role,omitempty"`
	Content   []ResponsesContentPart `json:"content,omitempty"`
	CallID    string                 `json:"call_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments string                 `json:"arguments,omitempty"`
	Extra     map[string]any         `json:"-"`
}

func (i ResponsesOutputItem) MarshalJSON() ([]byte, error) {
	type alias ResponsesOutputItem
	known, err := json.Marshal(alias(i))
	if err != nil {
		return nil, err
	}
	if len(i.Extra) == 0 {
		return known, nil
	}
	var fields map[string]any
	if err := json.Unmarshal(known, &fields); err != nil {
		return nil, err
	}
	for k, v := range i.Extra {
		if _, exists := fields[k]; !exists {
			fields[k] = v
		}
	}
	return json.Marshal(fields)
}

type ResponsesContentPart struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Refusal     string `json:"refusal,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

type ResponsesStreamEvent struct {
	Type         string               `json:"type,omitempty"`
	Response     *ResponsesResponse   `json:"response,omitempty"`
	Delta        string               `json:"delta,omitempty"`
	Item         *ResponsesOutputItem `json:"item,omitempty"`
	OutputIndex  *int                 `json:"output_index,omitempty"`
	ContentIndex *int                 `json:"content_index,omitempty"`
	SummaryIndex *int                 `json:"summary_index,omitempty"`
	ItemID       string               `json:"item_id,omitempty"`
	Part         json.RawMessage      `json:"part,omitempty"`
}
