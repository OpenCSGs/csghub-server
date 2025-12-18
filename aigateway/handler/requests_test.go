package handler

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
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
