package types

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/require"
)

// MultimodalContentProvider implementations must detect non-text content so
// capacity admission can skip the text-based TPM estimate for such requests.

func TestChatCompletionRequest_HasMultimodalContent(t *testing.T) {
	textOnly := &ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessageParamUnion{
			// Build via raw JSON to mirror real decoding paths.
		},
	}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]}
		]
	}`), textOnly))
	var _ MultimodalContentProvider = textOnly
	require.False(t, textOnly.HasMultimodalContent())

	withImage := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "what is this"},
				{"type": "image_url", "image_url": {"url": "https://example.com/a.png"}}
			]}
		]
	}`), withImage))
	require.True(t, withImage.HasMultimodalContent())

	withAudio := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o-audio",
		"messages": [
			{"role": "user", "content": [
				{"type": "input_audio", "input_audio": {"data": "abc", "format": "wav"}}
			]}
		]
	}`), withAudio))
	require.True(t, withAudio.HasMultimodalContent())

	var nilRequest *ChatCompletionRequest
	require.False(t, nilRequest.HasMultimodalContent())
}

func TestAnthropicMessagesRequest_HasMultimodalContent(t *testing.T) {
	textOnly := &AnthropicMessagesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "claude-3",
		"messages": [
			{"role": "user", "content": "plain text"}
		]
	}`), textOnly))
	var _ MultimodalContentProvider = textOnly
	require.False(t, textOnly.HasMultimodalContent())

	withImage := &AnthropicMessagesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "claude-3",
		"messages": [
			{"role": "user", "content": [
				{"type": "image", "source": {"type": "base64", "data": "abc"}}
			]}
		]
	}`), withImage))
	require.True(t, withImage.HasMultimodalContent())

	// Images nested inside tool_result blocks count too.
	nested := &AnthropicMessagesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "claude-3",
		"messages": [
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "t1", "content": [
					{"type": "image", "source": {"type": "base64", "data": "abc"}}
				]}
			]}
		]
	}`), nested))
	require.True(t, nested.HasMultimodalContent())
}

func TestResponsesRequest_HasMultimodalContent(t *testing.T) {
	textOnly := &ResponsesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": [
				{"type": "input_text", "text": "hello"}
			]}
		]
	}`), textOnly))
	var _ MultimodalContentProvider = textOnly
	require.False(t, textOnly.HasMultimodalContent())

	plainString := &ResponsesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"input": "hello"
	}`), plainString))
	require.False(t, plainString.HasMultimodalContent())

	withImage := &ResponsesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": [
				{"type": "input_text", "text": "what is this"},
				{"type": "input_image", "image_url": "https://example.com/a.png"}
			]}
		]
	}`), withImage))
	require.True(t, withImage.HasMultimodalContent())

	var nilRequest *ResponsesRequest
	require.False(t, nilRequest.HasMultimodalContent())
}
