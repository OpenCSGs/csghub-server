package types

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionRequest_MarshalUnmarshal(t *testing.T) {
	// Test case 1: Only known fields
	req1 := &ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hello"),
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	// Marshal to JSON
	data1, err := json.Marshal(req1)
	assert.NoError(t, err)

	// Unmarshal back
	var req1Unmarshaled ChatCompletionRequest
	err = json.Unmarshal(data1, &req1Unmarshaled)
	assert.NoError(t, err)

	// Verify fields
	assert.Equal(t, req1.Model, req1Unmarshaled.Model)
	assert.Equal(t, len(req1.Messages), len(req1Unmarshaled.Messages))
	assert.Equal(t, req1.Temperature, req1Unmarshaled.Temperature)
	assert.Equal(t, req1.MaxTokens, req1Unmarshaled.MaxTokens)
	assert.Empty(t, req1Unmarshaled.RawJSON)
}

func TestChatCompletionRequest_UnknownFields(t *testing.T) {
	// Test case 2: With unknown fields
	jsonWithUnknown := `{
		"model": "gpt-3.5-turbo",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.7,
		"max_tokens": 100,
		"unknown_field": "unknown_value",
		"another_unknown": 12345
	}`

	// Unmarshal
	var req2 ChatCompletionRequest
	err := json.Unmarshal([]byte(jsonWithUnknown), &req2)
	assert.NoError(t, err)

	// Verify known fields
	assert.Equal(t, "gpt-3.5-turbo", req2.Model)
	assert.Equal(t, 0.7, req2.Temperature)
	assert.Equal(t, 100, req2.MaxTokens)

	// Verify unknown fields are stored in RawJSON
	assert.NotEmpty(t, req2.RawJSON)

	// Marshal back and verify unknown fields are preserved
	data2, err := json.Marshal(req2)
	assert.NoError(t, err)

	// Unmarshal into map to check all fields
	var resultMap map[string]interface{}
	err = json.Unmarshal(data2, &resultMap)
	assert.NoError(t, err)

	// Check known fields
	assert.Equal(t, "gpt-3.5-turbo", resultMap["model"])
	assert.Equal(t, 0.7, resultMap["temperature"])
	assert.Equal(t, 100.0, resultMap["max_tokens"])

	// Check unknown fields
	assert.Equal(t, "unknown_value", resultMap["unknown_field"])
	assert.Equal(t, 12345.0, resultMap["another_unknown"])
}

func TestChatCompletionRequest_ComplexUnknownFields(t *testing.T) {
	// Test case 3: With complex unknown fields
	jsonWithComplexUnknown := `{
		"model": "gpt-3.5-turbo",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"stream": true,
		"complex_field": {
			"nested1": "value1",
			"nested2": {
				"deep": 123
			}
		},
		"array_field": [1, 2, 3, 4, 5]
	}`

	// Unmarshal
	var req3 ChatCompletionRequest
	err := json.Unmarshal([]byte(jsonWithComplexUnknown), &req3)
	assert.NoError(t, err)

	// Verify known fields
	assert.Equal(t, "gpt-3.5-turbo", req3.Model)
	assert.True(t, req3.Stream)

	// Verify unknown fields are stored
	assert.NotEmpty(t, req3.RawJSON)

	// Marshal back and verify all fields are preserved
	data3, err := json.Marshal(req3)
	assert.NoError(t, err)

	// Unmarshal into map to check
	var resultMap map[string]interface{}
	err = json.Unmarshal(data3, &resultMap)
	assert.NoError(t, err)

	// Check known fields
	assert.Equal(t, "gpt-3.5-turbo", resultMap["model"])
	assert.True(t, resultMap["stream"].(bool))

	// Check complex unknown fields
	complexField, ok := resultMap["complex_field"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value1", complexField["nested1"])

	nested2, ok := complexField["nested2"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 123.0, nested2["deep"])

	// Check array unknown field
	arrayField, ok := resultMap["array_field"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, arrayField, 5)
	assert.Equal(t, 1.0, arrayField[0])
	assert.Equal(t, 5.0, arrayField[4])
}

func TestChatCompletionRequest_EmptyRawJSON(t *testing.T) {
	// Test case 4: Empty RawJSON should not cause issues
	req4 := &ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hello"),
		},
		RawJSON: nil,
	}

	// Marshal should work fine
	data4, err := json.Marshal(req4)
	assert.NoError(t, err)

	// Unmarshal should work fine
	var req4Unmarshaled ChatCompletionRequest
	err = json.Unmarshal(data4, &req4Unmarshaled)
	assert.NoError(t, err)

	// RawJSON should be empty
	assert.Empty(t, req4Unmarshaled.RawJSON)
}

func TestChatCompletionRequest_HasNullMessageContent(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected bool
	}{
		{
			name:     "assistant message with explicit null content",
			raw:      `{"messages": [{"role": "assistant", "content": null, "tool_calls": [{"id": "call_1"}]}]}`,
			expected: true,
		},
		{
			name:     "user message with explicit null content",
			raw:      `{"messages": [{"role": "user", "content": null}]}`,
			expected: true,
		},
		{
			name:     "missing content key does not count",
			raw:      `{"messages": [{"role": "assistant", "tool_calls": [{"id": "call_1"}]}]}`,
			expected: false,
		},
		{
			name:     "string content does not count",
			raw:      `{"messages": [{"role": "user", "content": "hello"}]}`,
			expected: false,
		},
		{
			name:     "null in other fields does not count",
			raw:      `{"messages": [{"role": "user", "content": "hello"}], "tool_choice": null}`,
			expected: false,
		},
		{
			name:     "no messages",
			raw:      `{"model": "m"}`,
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ChatCompletionRequest
			require.NoError(t, json.Unmarshal([]byte(tt.raw), &req))
			assert.Equal(t, tt.expected, req.HasNullMessageContent())
		})
	}
}

func TestChatCompletionRequest_AssistantReasoningContentRoundTrip(t *testing.T) {
	raw := []byte(`{
		"model": "deepseek-v4-flash",
		"thinking": {"type": "enabled"},
		"messages": [
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": "", "reasoning_content": "thinking hard", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
		],
		"tools": [{"type": "function", "function": {"name": "get_weather"}}]
	}`)

	var req ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	data, err := json.Marshal(&req)
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &result))

	// reasoning_content on assistant messages must survive the round trip:
	// thinking-mode providers require it on subsequent requests.
	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(result["messages"], &messages))
	require.Len(t, messages, 3)

	var reasoning string
	require.NoError(t, json.Unmarshal(messages[1]["reasoning_content"], &reasoning))
	assert.Equal(t, "thinking hard", reasoning)

	// the tool message must not gain a reasoning_content field
	assert.NotContains(t, string(result["messages"][2]), "reasoning_content")

	// the top-level thinking field must survive as well
	assert.JSONEq(t, `{"type": "enabled"}`, string(result["thinking"]))
}

func TestChatCompletionRequest_RoundTripWithoutReasoningContent(t *testing.T) {
	raw := []byte(`{
		"model": "deepseek-v4-flash",
		"messages": [
			{"role": "assistant", "content": "answer"}
		]
	}`)

	var req ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	data, err := json.Marshal(&req)
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &result))
	assert.NotContains(t, string(result["messages"]), "reasoning_content")
}

func TestModelRequiresToolCallReasoningContent(t *testing.T) {
	assert.True(t, ModelRequiresToolCallReasoningContent("deepseek-v4-flash"))
	assert.True(t, ModelRequiresToolCallReasoningContent("DeepSeek-Flash"))
	assert.True(t, ModelRequiresToolCallReasoningContent("kimi-k2-0905-preview"))
	assert.True(t, ModelRequiresToolCallReasoningContent("moonshot-v1-8k"))
	assert.False(t, ModelRequiresToolCallReasoningContent("gpt-4o"))
	assert.False(t, ModelRequiresToolCallReasoningContent(""))
}

func TestInjectToolCallReasoningContent(t *testing.T) {
	t.Run("InjectsEmptyReasoningContentOnToolCallMessages", func(t *testing.T) {
		body := []byte(`{
			"model": "deepseek-flash",
			"messages": [
				{"role": "user", "content": "hi"},
				{"role": "assistant", "content": "Checking.", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "f"}}]},
				{"role": "assistant", "content": "final answer", "tool_calls": null}
			]
		}`)
		out := InjectToolCallReasoningContent(body)

		var payload struct {
			Messages []struct {
				Role             string  `json:"role"`
				ReasoningContent *string `json:"reasoning_content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(out, &payload))
		require.Len(t, payload.Messages, 3)
		require.NotNil(t, payload.Messages[1].ReasoningContent)
		assert.Empty(t, *payload.Messages[1].ReasoningContent)
		assert.Nil(t, payload.Messages[0].ReasoningContent, "user messages must not be modified")
		assert.Nil(t, payload.Messages[2].ReasoningContent, "tool_calls: null must not be modified")
	})

	t.Run("KeepsExistingReasoningContent", func(t *testing.T) {
		body := []byte(`{"messages": [{"role": "assistant", "reasoning_content": "real reasoning", "tool_calls": [{"id": "call_1"}]}]}`)
		out := InjectToolCallReasoningContent(body)
		var payload struct {
			Messages []struct {
				ReasoningContent string `json:"reasoning_content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(out, &payload))
		assert.Equal(t, "real reasoning", payload.Messages[0].ReasoningContent)
	})

	t.Run("OverwritesNullReasoningContentWithEmptyString", func(t *testing.T) {
		body := []byte(`{"messages": [{"role": "assistant", "reasoning_content": null, "tool_calls": [{"id": "call_1"}]}]}`)
		out := InjectToolCallReasoningContent(body)
		var payload struct {
			Messages []struct {
				ReasoningContent *string `json:"reasoning_content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(out, &payload))
		require.NotNil(t, payload.Messages[0].ReasoningContent,
			"explicit null must be treated as missing and replaced with an empty string")
		assert.Empty(t, *payload.Messages[0].ReasoningContent)
	})

	t.Run("Idempotent", func(t *testing.T) {
		body := []byte(`{"messages": [{"role": "assistant", "tool_calls": [{"id": "call_1"}]}]}`)
		once := InjectToolCallReasoningContent(body)
		twice := InjectToolCallReasoningContent(once)
		assert.JSONEq(t, string(once), string(twice))
	})

	t.Run("NoMessagesKeyReturnsBodyUnchanged", func(t *testing.T) {
		body := []byte(`{"model": "deepseek-flash"}`)
		assert.Equal(t, body, InjectToolCallReasoningContent(body))
	})
}

func TestEmbeddingRequest_MarshalUnmarshal(t *testing.T) {
	// Test case 1: Only known fields
	req1 := &EmbeddingRequest{
		EmbeddingNewParams: openai.EmbeddingNewParams{
			Input: openai.EmbeddingNewParamsInputUnion{
				OfArrayOfStrings: []string{"Hello, world!"},
			},
			Model: "text-embedding-ada-002",
		},
	}

	// Marshal to JSON
	data1, err := json.Marshal(req1)
	assert.NoError(t, err)

	// Unmarshal back
	var req1Unmarshaled EmbeddingRequest
	err = json.Unmarshal(data1, &req1Unmarshaled)
	assert.NoError(t, err)

	// Verify fields
	assert.Equal(t, req1.Model, req1Unmarshaled.Model)
	assert.Equal(t, len(req1.Input.OfArrayOfStrings), len(req1Unmarshaled.Input.OfArrayOfStrings))
	assert.Empty(t, req1Unmarshaled.RawJSON)
}

func TestEmbeddingRequest_UnknownFields(t *testing.T) {
	// Test case 2: With unknown fields
	jsonWithUnknown := `{
		"model": "text-embedding-ada-002",
		"input": ["Hello, world!"],
		"unknown_field": "unknown_value",
		"another_unknown": 12345
	}`

	// Unmarshal
	var req2 EmbeddingRequest
	err := json.Unmarshal([]byte(jsonWithUnknown), &req2)
	assert.NoError(t, err)

	// Verify known fields
	assert.Equal(t, "text-embedding-ada-002", req2.Model)
	assert.Equal(t, 1, len(req2.Input.OfArrayOfStrings))

	// Verify unknown fields are stored in RawJSON
	assert.NotEmpty(t, req2.RawJSON)

	// Marshal back and verify unknown fields are preserved
	data2, err := json.Marshal(req2)
	assert.NoError(t, err)

	// Unmarshal into map to check all fields
	var resultMap map[string]interface{}
	err = json.Unmarshal(data2, &resultMap)
	assert.NoError(t, err)

	// Check known fields
	assert.Equal(t, "text-embedding-ada-002", resultMap["model"])
	inputArray, ok := resultMap["input"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Hello, world!", inputArray[0])

	// Check unknown fields
	assert.Equal(t, "unknown_value", resultMap["unknown_field"])
	assert.Equal(t, 12345.0, resultMap["another_unknown"])
}
func TestImageGenerationRequest_MarshalUnmarshal(t *testing.T) {
	// Test case 1: Only known fields
	t.Run("OnlyKnownFields", func(t *testing.T) {
		// Create a request with only known fields
		original := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "dall-e-3",
				Prompt: "A beautiful sunset",
				Size:   openai.ImageGenerateParamsSize1024x1024,
			},
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(original)
		require.NoError(t, err)

		// Unmarshal back
		var unmarshaled ImageGenerationRequest
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		// Check that known fields are preserved
		assert.Equal(t, original.Model, unmarshaled.Model)
		assert.Equal(t, original.Prompt, unmarshaled.Prompt)
		assert.Equal(t, original.Size, unmarshaled.Size)
		// RawJSON should be nil
		assert.Nil(t, unmarshaled.RawJSON)
	})

	// Test case 2: Only unknown fields
	t.Run("OnlyUnknownFields", func(t *testing.T) {
		// JSON with only unknown fields
		jsonData := []byte(`{"custom_field": "value", "another_custom": 123}`)

		// Unmarshal
		var unmarshaled ImageGenerationRequest
		err := json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		// Check that known fields are zero values
		assert.Empty(t, unmarshaled.Model)
		assert.Empty(t, unmarshaled.Prompt)
		assert.Empty(t, unmarshaled.Size)

		// Check that unknown fields are stored in RawJSON
		assert.NotNil(t, unmarshaled.RawJSON)
		assert.JSONEq(t, string(jsonData), string(unmarshaled.RawJSON))

		// Marshal back and check
		marshaledData, err := json.Marshal(unmarshaled)
		require.NoError(t, err)
		// must contain all unknown fields and required fields
		assert.Contains(t, string(marshaledData), "custom_field")
		assert.Contains(t, string(marshaledData), "another_custom")
		assert.Contains(t, string(marshaledData), "prompt")
	})

	// Test case 3: Both known and unknown fields
	t.Run("KnownAndUnknownFields", func(t *testing.T) {
		// JSON with both known and unknown fields
		jsonData := []byte(`{
			"model": "dall-e-3",
			"prompt": "A beautiful sunset",
			"size": "1024x1024",
			"custom_field": "value",
			"another_custom": 123
		}`)

		// Unmarshal
		var unmarshaled ImageGenerationRequest
		err := json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		// Check that known fields are correctly unmarshaled
		assert.Equal(t, "dall-e-3", unmarshaled.Model)
		assert.Equal(t, "A beautiful sunset", unmarshaled.Prompt)
		assert.Equal(t, openai.ImageGenerateParamsSize1024x1024, unmarshaled.Size)

		// Check that unknown fields are stored in RawJSON
		assert.NotNil(t, unmarshaled.RawJSON)
		assert.JSONEq(t, `{"custom_field": "value", "another_custom": 123}`, string(unmarshaled.RawJSON))

		// Marshal back and check
		marshaledData, err := json.Marshal(unmarshaled)
		require.NoError(t, err)
		assert.JSONEq(t, string(jsonData), string(marshaledData))
	})

	// Test case 4: Unknown fields override known fields during marshal
	t.Run("UnknownFieldsOverride", func(t *testing.T) {
		// Create a request with known fields
		request := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "dall-e-3",
				Prompt: "A beautiful sunset",
			},
			// Add unknown field that overrides a known field
			RawJSON: []byte(`{"prompt": "Overridden prompt"}`),
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(request)
		require.NoError(t, err)

		// Check that unknown field overrides known field in output
		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)
		assert.Equal(t, "Overridden prompt", result["prompt"])
		assert.Equal(t, "dall-e-3", result["model"])
	})

	// Test case 5: Empty RawJSON
	t.Run("EmptyRawJSON", func(t *testing.T) {
		// Create a request with empty RawJSON
		request := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "dall-e-3",
				Prompt: "A beautiful sunset",
			},
			RawJSON: []byte(`{}`),
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(request)
		require.NoError(t, err)

		// Check that output doesn't include empty object fields
		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)
		assert.Equal(t, "dall-e-3", result["model"])
		assert.Equal(t, "A beautiful sunset", result["prompt"])
		// Should not have any extra fields
		assert.Len(t, result, 2) // model and prompt
	})
}
