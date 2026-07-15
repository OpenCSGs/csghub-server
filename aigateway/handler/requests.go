package handler

import (
	"encoding/json"

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
	return nil
}

// MarshalJSON implements json.Marshaler interface
func (r ChatCompletionRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	type TempChatCompletionRequest ChatCompletionRequest
	data, err := json.Marshal(TempChatCompletionRequest(r))
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

// ImageGenerationRequest represents an image generation request structure
type ImageGenerationRequest struct {
	openai.ImageGenerateParams
	RawJSON json.RawMessage `json:"-"` // Raw JSON data for fields not explicitly defined in the struct
}

// UnmarshalJSON implements the json.Unmarshaler interface to handle undefined fields
func (r *ImageGenerationRequest) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a map to capture all fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Then, unmarshal the known fields into the struct
	knownFieldsData, err := json.Marshal(rawMap)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(knownFieldsData, &r.ImageGenerateParams); err != nil {
		return err
	}

	// Remove the known fields from the map
	// We need to know what fields are in openai.ImageGenerateParams
	// For simplicity, we'll re-encode the struct and decode it back to a map
	// This way we can get all the known field names
	var knownFields map[string]interface{}
	if knownData, err := json.Marshal(r.ImageGenerateParams); err == nil {
		if err := json.Unmarshal(knownData, &knownFields); err == nil {
			// Remove known fields from rawMap
			for k := range knownFields {
				delete(rawMap, k)
			}
		}
	}

	// Marshal the remaining unknown fields back to JSON
	if len(rawMap) > 0 {
		rawJSON, err := json.Marshal(rawMap)
		if err != nil {
			return err
		}
		r.RawJSON = rawJSON
	} else {
		r.RawJSON = nil
	}

	return nil
}

// MarshalJSON implements the json.Marshaler interface to include undefined fields
func (r ImageGenerationRequest) MarshalJSON() ([]byte, error) {
	// First, marshal the known fields
	knownData, err := json.Marshal(r.ImageGenerateParams)
	if err != nil {
		return nil, err
	}

	// If there are no unknown fields, just return the known fields
	if len(r.RawJSON) == 0 {
		return knownData, nil
	}

	// Unmarshal both known and unknown fields into maps
	var knownMap, unknownMap map[string]interface{}
	if err := json.Unmarshal(knownData, &knownMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.RawJSON, &unknownMap); err != nil {
		return nil, err
	}

	// Merge the maps, unknown fields override known fields
	for k, v := range unknownMap {
		knownMap[k] = v
	}

	// Marshal the merged map back to JSON
	return json.Marshal(knownMap)
}
