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
