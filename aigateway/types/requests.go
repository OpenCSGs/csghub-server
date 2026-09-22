package types

import (
	"encoding/json"
	"strings"

	"github.com/openai/openai-go/v3"
)

// ChatCompletionRequest represents a chat completion request
//
// refer to openai.ChatCompletionNewParams in
// https://github.com/openai/openai-go/blob/main/chatcompletion.go#L2902
type ChatCompletionRequest struct {
	Model    string                                   `json:"model"`
	Messages []openai.ChatCompletionMessageParamUnion `json:"messages"`
	// Controls which (if any) tool is called by the model. `none` means the model will
	// not call any tool and instead generates a message. `auto` means the model can
	// pick between generating a message or calling one or more tools. `required` means
	// the model must call one or more tools. Specifying a particular tool via
	// `{"type": "function", "function": {"name": "my_function"}}` forces the model to
	// call that tool.
	//
	// `none` is the default when no tools are present. `auto` is the default if tools
	// are present.
	ToolChoice openai.ChatCompletionToolChoiceOptionUnionParam `json:"tool_choice,omitzero"`
	// A list of tools the model may call. You can provide either
	// [custom tools](https://platform.openai.com/docs/guides/function-calling#custom-tools)
	// or [function tools](https://platform.openai.com/docs/guides/function-calling).
	Tools         []openai.ChatCompletionToolUnionParam `json:"tools,omitzero"`
	Temperature   float64                               `json:"temperature,omitempty"`
	MaxTokens     int                                   `json:"max_tokens,omitempty"`
	TopP          float64                               `json:"top_p,omitempty"`
	Stream        bool                                  `json:"stream,omitempty"`
	StreamOptions *StreamOptions                        `json:"stream_options,omitempty"`
	// RawJSON stores all unknown fields during unmarshaling
	RawJSON json.RawMessage `json:"-"`
	// RawBody keeps the client's original request bytes. The native chat
	// completions path proxies this body with only the fields the gateway
	// must override (model, streaming usage option), so vendor-specific
	// fields inside messages or tools survive the round trip unchanged.
	RawBody json.RawMessage `json:"-"`
	// ClientModel is the model name exactly as the client sent it. The
	// handler overwrites Model with the resolved upstream name, so this
	// field is what tells the raw-body fast path whether a model rewrite
	// is actually needed.
	ClientModel string `json:"-"`
	// ForceStreamUsage records that the gateway itself decided to force the
	// stream_options.include_usage option on this streaming request, because
	// usage metering depends on the final usage chunk. A client-provided
	// stream_options value is forwarded verbatim when this is not set.
	ForceStreamUsage bool `json:"-"`
	// assistantReasoningContent keeps the `reasoning_content` value of
	// assistant messages by their index in the messages array. The openai-go
	// message param types do not model this field, so without it a client
	// that correctly passes reasoning content back would lose it when the
	// gateway re-serializes the request. Thinking-mode providers such as
	// DeepSeek require the field on subsequent requests.
	assistantReasoningContent map[int]string
	// hasNullMessageContent records whether any message carries an explicit
	// `"content": null` (a common SDK idiom on assistant tool-call turns).
	// Strict upstreams reject the field, so the raw-body proxy path must
	// rewrite those messages instead of taking the zero-cost fast path.
	hasNullMessageContent bool
}

// PromptText extracts a plain-text representation of the user's prompt from
// the chat messages, joining all message content with newlines.  This
// satisfies types.PromptTextProvider so the Planner can perform
// sensitive-content checks without depending on the concrete message type.
// The extraction mirrors the logic in moderation.CheckChatPrompts.
func (r *ChatCompletionRequest) PromptText() string {
	if r == nil || len(r.Messages) == 0 {
		return ""
	}
	var b strings.Builder
	for _, msg := range r.Messages {
		switch rawContent := msg.GetContent().AsAny().(type) {
		case string:
			b.WriteString(rawContent)
			b.WriteByte('\n')
		case *string:
			b.WriteString(*rawContent)
			b.WriteByte('\n')
		case []interface{}:
			for _, item := range rawContent {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if text, exists := itemMap["text"].(string); exists {
						b.WriteString(text)
						b.WriteByte(' ')
					}
				}
			}
			b.WriteByte('\n')
		default:
			contentBytes, _ := json.Marshal(rawContent)
			if len(contentBytes) > 0 {
				b.Write(contentBytes)
				b.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// HasMultimodalContent reports whether any message carries non-text content
// parts (images, audio, files). It satisfies types.MultimodalContentProvider
// so capacity admission can skip the text-based TPM estimate: a text-only
// estimate is fiction for such requests.
func (r *ChatCompletionRequest) HasMultimodalContent() bool {
	if r == nil {
		return false
	}
	for _, msg := range r.Messages {
		switch content := msg.GetContent().AsAny().(type) {
		case *[]openai.ChatCompletionContentPartUnionParam:
			for _, part := range *content {
				if part.OfImageURL != nil || part.OfInputAudio != nil || part.OfFile != nil {
					return true
				}
			}
		case []interface{}:
			// Generic decode path: content parts are raw maps.
			for _, part := range content {
				partMap, ok := part.(map[string]interface{})
				if !ok {
					continue
				}
				if partType, _ := partMap["type"].(string); partType != "" && partType != "text" {
					return true
				}
			}
		}
	}
	return false
}

// UnmarshalJSON implements json.Unmarshaler interface
func (r *ChatCompletionRequest) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to hold the known fields
	type TempChatCompletionRequest ChatCompletionRequest

	// First, unmarshal into the temporary struct
	var temp TempChatCompletionRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Then, unmarshal into a map to get all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Keep the raw messages array so unmodeled message fields (e.g.
	// reasoning_content on assistant messages) can be restored on marshal.
	messagesRaw := allFields["messages"]

	// Remove known fields from the map
	delete(allFields, "model")
	delete(allFields, "messages")
	delete(allFields, "tool_choice")
	delete(allFields, "tools")
	delete(allFields, "temperature")
	delete(allFields, "max_tokens")
	delete(allFields, "top_p")
	delete(allFields, "stream")
	delete(allFields, "stream_options")

	// If there are any unknown fields left, marshal them into RawJSON
	var rawJSON []byte
	var err error
	if len(allFields) > 0 {
		rawJSON, err = json.Marshal(allFields)
		if err != nil {
			return err
		}
	}

	// Assign the temporary struct to the original and set RawJSON
	*r = ChatCompletionRequest(temp)
	r.RawJSON = rawJSON
	// Copy the original bytes: the decoder may reuse its internal buffer,
	// and the raw body is proxied to the upstream after the request body
	// has been consumed.
	r.RawBody = append(json.RawMessage(nil), data...)
	// Model still holds the client's value here; the handler overwrites it
	// with the resolved upstream name before proxying.
	r.ClientModel = r.Model
	r.assistantReasoningContent = collectAssistantReasoningContent(messagesRaw)
	r.hasNullMessageContent = hasNullMessageContent(messagesRaw)
	return nil
}

// hasNullMessageContent reports whether any message carries an explicit
// `"content": null`. Some SDKs send it on assistant messages that only carry
// tool calls; strict upstreams (e.g. DeepSeek) reject the field and expect
// the key to be absent instead, so the raw-body proxy path must rewrite
// those messages. A missing content key does not count.
func hasNullMessageContent(messagesRaw json.RawMessage) bool {
	if len(messagesRaw) == 0 {
		return false
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return false
	}
	for _, msgRaw := range messages {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		// A JSON null decodes into a *json.RawMessage pointer the same way a
		// missing key does, so presence must be checked on the raw map.
		if content, ok := msg["content"]; ok && IsJSONNull(content) {
			return true
		}
	}
	return false
}

// HasNullMessageContent reports whether any message in the parsed request
// carries an explicit `"content": null` value.
func (r *ChatCompletionRequest) HasNullMessageContent() bool {
	return r.hasNullMessageContent
}

// collectAssistantReasoningContent extracts the `reasoning_content` value of
// assistant messages from the raw messages array. It returns nil when no
// assistant message carries the field or the payload has an unexpected shape.
func collectAssistantReasoningContent(messagesRaw json.RawMessage) map[int]string {
	if len(messagesRaw) == 0 {
		return nil
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return nil
	}
	result := make(map[int]string)
	for i, msgRaw := range messages {
		var msg struct {
			Role             string  `json:"role"`
			ReasoningContent *string `json:"reasoning_content"`
		}
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		if msg.Role != "assistant" || msg.ReasoningContent == nil {
			continue
		}
		result[i] = *msg.ReasoningContent
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// MarshalJSON implements json.Marshaler interface
func (r ChatCompletionRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempChatCompletionRequest ChatCompletionRequest
	data, err := json.Marshal(TempChatCompletionRequest(r))
	if err != nil {
		return nil, err
	}

	// If there is nothing to merge back, just return the known fields
	if len(r.RawJSON) == 0 && len(r.assistantReasoningContent) == 0 {
		return data, nil
	}

	// Parse the known fields back into a map
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	// Parse the raw JSON fields into a map
	var rawFields map[string]json.RawMessage
	if len(r.RawJSON) > 0 {
		if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
			return nil, err
		}
	}

	// Merge the raw fields into the known fields
	for k, v := range rawFields {
		knownFields[k] = v
	}

	restoreAssistantReasoningContent(knownFields, r.assistantReasoningContent)

	// Marshal the merged map back into JSON
	return json.Marshal(knownFields)
}

// restoreAssistantReasoningContent merges the `reasoning_content` values
// captured during unmarshaling back onto the marshaled assistant messages,
// by their index in the messages array. Messages are only re-serialized when
// a value actually needs restoring. It relies on the messages array not
// being reordered between unmarshal and marshal, which holds for the
// gateway's chat completions path.
func restoreAssistantReasoningContent(knownFields map[string]json.RawMessage, reasoning map[int]string) {
	if len(reasoning) == 0 {
		return
	}
	messagesRaw, ok := knownFields["messages"]
	if !ok {
		return
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return
	}
	changed := false
	for i, msgRaw := range messages {
		value, ok := reasoning[i]
		if !ok {
			continue
		}
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		if _, exists := msg["reasoning_content"]; exists {
			continue
		}
		var role string
		if roleRaw, ok := msg["role"]; ok {
			_ = json.Unmarshal(roleRaw, &role)
		}
		if role != "assistant" {
			continue
		}
		valueJSON, err := json.Marshal(value)
		if err != nil {
			continue
		}
		msg["reasoning_content"] = valueJSON
		updated, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		messages[i] = updated
		changed = true
	}
	if !changed {
		return
	}
	if merged, err := json.Marshal(messages); err == nil {
		knownFields["messages"] = merged
	}
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// EmbeddingRequest represents an embedding request structure
type EmbeddingRequest struct {
	openai.EmbeddingNewParams
	// RawJSON stores all unknown fields during unmarshaling
	RawJSON json.RawMessage `json:"-"`
}

func (r *EmbeddingRequest) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to hold the known fields
	type TempEmbeddingRequest EmbeddingRequest

	// First, unmarshal into the temporary struct
	var temp TempEmbeddingRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Then, unmarshal into a map to get all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Remove known fields from the map
	delete(allFields, "model")
	delete(allFields, "input")
	delete(allFields, "encoding_format")

	// If there are any unknown fields left, marshal them into RawJSON
	var rawJSON []byte
	var err error
	if len(allFields) > 0 {
		rawJSON, err = json.Marshal(allFields)
		if err != nil {
			return err
		}
	}

	// Assign the temporary struct to the original and set RawJSON
	*r = EmbeddingRequest(temp)
	r.RawJSON = rawJSON
	return nil
}

func (r EmbeddingRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempEmbeddingRequest EmbeddingRequest
	data, err := json.Marshal(TempEmbeddingRequest(r))
	if err != nil {
		return nil, err
	}

	// If there are no raw JSON fields, just return the known fields
	if len(r.RawJSON) == 0 {
		return data, nil
	}

	// Parse the known fields back into a map
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	// Parse the raw JSON fields into a map
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
		return nil, err
	}

	// Merge the raw fields into the known fields
	for k, v := range rawFields {
		knownFields[k] = v
	}

	// Marshal the merged map back into JSON
	return json.Marshal(knownFields)
}

// RerankRequest represents a rerank request (Jina/Cohere compatible API,
// served by vllm, TEI and llama.cpp for text-ranking models)
type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int64    `json:"top_n,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`
	// RawJSON stores all unknown fields during unmarshaling
	RawJSON json.RawMessage `json:"-"`
}

func (r *RerankRequest) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to hold the known fields
	type TempRerankRequest RerankRequest

	// First, unmarshal into the temporary struct
	var temp TempRerankRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Then, unmarshal into a map to get all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Remove known fields from the map
	delete(allFields, "model")
	delete(allFields, "query")
	delete(allFields, "documents")
	delete(allFields, "top_n")
	delete(allFields, "return_documents")

	// If there are any unknown fields left, marshal them into RawJSON
	var rawJSON []byte
	var err error
	if len(allFields) > 0 {
		rawJSON, err = json.Marshal(allFields)
		if err != nil {
			return err
		}
	}

	// Assign the temporary struct to the original and set RawJSON
	*r = RerankRequest(temp)
	r.RawJSON = rawJSON
	return nil
}

func (r RerankRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempRerankRequest RerankRequest
	data, err := json.Marshal(TempRerankRequest(r))
	if err != nil {
		return nil, err
	}

	// If there are no raw JSON fields, just return the known fields
	if len(r.RawJSON) == 0 {
		return data, nil
	}

	// Parse the known fields back into a map
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	// Parse the raw JSON fields into a map
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
		return nil, err
	}

	// Merge the raw fields into the known fields
	for k, v := range rawFields {
		knownFields[k] = v
	}

	// Marshal the merged map back into JSON
	return json.Marshal(knownFields)
}

// SpeechRequest represents an OpenAI-compatible text-to-speech request
// (POST /v1/audio/speech, served by vLLM-Omni and other TTS backends).
// Backend-specific extension fields (task_type, language, instructions,
// ref_audio, ref_text, ...) are preserved in RawJSON and passed through.
type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
	StreamFormat   string  `json:"stream_format,omitempty"`
	// RawJSON stores all unknown fields during unmarshaling
	RawJSON json.RawMessage `json:"-"`
}

func (r *SpeechRequest) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to hold the known fields
	type TempSpeechRequest SpeechRequest

	// First, unmarshal into the temporary struct
	var temp TempSpeechRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Then, unmarshal into a map to get all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Remove known fields from the map
	delete(allFields, "model")
	delete(allFields, "input")
	delete(allFields, "voice")
	delete(allFields, "response_format")
	delete(allFields, "speed")
	delete(allFields, "stream")
	delete(allFields, "stream_format")

	// If there are any unknown fields left, marshal them into RawJSON
	var rawJSON []byte
	var err error
	if len(allFields) > 0 {
		rawJSON, err = json.Marshal(allFields)
		if err != nil {
			return err
		}
	}

	// Assign the temporary struct to the original and set RawJSON
	*r = SpeechRequest(temp)
	r.RawJSON = rawJSON
	return nil
}

func (r SpeechRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempSpeechRequest SpeechRequest
	data, err := json.Marshal(TempSpeechRequest(r))
	if err != nil {
		return nil, err
	}

	// If there are no raw JSON fields, just return the known fields
	if len(r.RawJSON) == 0 {
		return data, nil
	}

	// Parse the known fields back into a map
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	// Parse the raw JSON fields into a map
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
		return nil, err
	}

	// Merge the raw fields into the known fields
	for k, v := range rawFields {
		knownFields[k] = v
	}

	// Marshal the merged map back into JSON
	return json.Marshal(knownFields)
}

// BatchSpeechRequest represents an OpenAI-compatible batch text-to-speech
// request (POST /v1/audio/speech/batch). Items and batch-level defaults are
// passed through unchanged; only the model field is rewritten.
type BatchSpeechRequest struct {
	Model string            `json:"model"`
	Items []json.RawMessage `json:"items"`
	// RawJSON stores all unknown fields during unmarshaling
	RawJSON json.RawMessage `json:"-"`
}

// InputTexts extracts the text of every item for moderation and fallback
// billing purposes.
func (r *BatchSpeechRequest) InputTexts() []string {
	texts := make([]string, 0, len(r.Items))
	for _, item := range r.Items {
		var parsed struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(item, &parsed); err != nil {
			continue
		}
		if parsed.Input != "" {
			texts = append(texts, parsed.Input)
		}
	}
	return texts
}

func (r *BatchSpeechRequest) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to hold the known fields
	type TempBatchSpeechRequest BatchSpeechRequest

	// First, unmarshal into the temporary struct
	var temp TempBatchSpeechRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Then, unmarshal into a map to get all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Remove known fields from the map
	delete(allFields, "model")
	delete(allFields, "items")

	// If there are any unknown fields left, marshal them into RawJSON
	var rawJSON []byte
	var err error
	if len(allFields) > 0 {
		rawJSON, err = json.Marshal(allFields)
		if err != nil {
			return err
		}
	}

	// Assign the temporary struct to the original and set RawJSON
	*r = BatchSpeechRequest(temp)
	r.RawJSON = rawJSON
	return nil
}

func (r BatchSpeechRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempBatchSpeechRequest BatchSpeechRequest
	data, err := json.Marshal(TempBatchSpeechRequest(r))
	if err != nil {
		return nil, err
	}

	// If there are no raw JSON fields, just return the known fields
	if len(r.RawJSON) == 0 {
		return data, nil
	}

	// Parse the known fields back into a map
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	// Parse the raw JSON fields into a map
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
		return nil, err
	}

	// Merge the raw fields into the known fields
	for k, v := range rawFields {
		knownFields[k] = v
	}

	// Marshal the merged map back into JSON
	return json.Marshal(knownFields)
}
