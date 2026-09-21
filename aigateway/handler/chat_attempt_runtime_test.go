package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/aigateway/types"
)

// toolCallChatRequest builds a chat completion request the way the gateway
// receives it: an assistant tool-call turn without reasoning_content followed
// by the tool result, plus the tools parameter.
func toolCallChatRequest(t *testing.T, modelName string, extra string) *types.ChatCompletionRequest {
	t.Helper()
	raw := `{
		"model": "` + modelName + `",
		"messages": [
			{"role": "user", "content": "What is the weather in Beijing?"},
			{"role": "assistant", "content": "Checking.", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Beijing\"}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
		],
		"tools": [{"type": "function", "function": {"name": "get_weather"}}]` + extra + `}`
	req := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(raw), req))
	return req
}

func messageReasoningContent(t *testing.T, message json.RawMessage) (string, bool) {
	t.Helper()
	var msg struct {
		Role             string  `json:"role"`
		ReasoningContent *string `json:"reasoning_content"`
	}
	require.NoError(t, json.Unmarshal(message, &msg))
	if msg.ReasoningContent == nil {
		return "", false
	}
	return *msg.ReasoningContent, true
}

func decodeMessages(t *testing.T, body []byte) []json.RawMessage {
	t.Helper()
	var payload struct {
		Messages []json.RawMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload.Messages
}

func TestMarshalChatRequestBodyInjectsToolCallReasoningContent(t *testing.T) {
	chatReq := toolCallChatRequest(t, "deepseek-v4-flash", "")

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	reasoning, ok := messageReasoningContent(t, messages[1])
	require.True(t, ok, "assistant tool-call message must carry reasoning_content")
	assert.Empty(t, reasoning, "missing reasoning_content must be filled with an empty string")
	_, ok = messageReasoningContent(t, messages[0])
	assert.False(t, ok, "user messages must not gain reasoning_content")
}

func TestMarshalChatRequestBodyKeepsExistingReasoningContent(t *testing.T) {
	// Simulate a client that correctly passes reasoning_content back.
	raw := []byte(`{
		"model": "deepseek-v4-flash",
		"messages": [
			{"role": "user", "content": "What is the weather in Beijing?"},
			{"role": "assistant", "content": "Checking.", "reasoning_content": "I should call get_weather.", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Beijing\"}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
		],
		"tools": [{"type": "function", "function": {"name": "get_weather"}}]
	}`)
	chatReq := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal(raw, chatReq))

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	reasoning, ok := messageReasoningContent(t, messages[1])
	require.True(t, ok)
	assert.Equal(t, "I should call get_weather.", reasoning)
}

func TestMarshalChatRequestBodySkipsNonDeepSeekModels(t *testing.T) {
	chatReq := toolCallChatRequest(t, "gpt-4o", "")

	body, err := marshalChatRequestBody(chatReq, "gpt-4o")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	_, ok := messageReasoningContent(t, messages[1])
	assert.False(t, ok, "non deepseek-family models must not be modified")
}

func TestMarshalChatRequestBodySkipsWhenToolsAbsent(t *testing.T) {
	chatReq := toolCallChatRequest(t, "deepseek-v4-flash", "")
	chatReq.Tools = nil

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	_, ok := messageReasoningContent(t, messages[1])
	assert.False(t, ok, "requests without tools must not be modified")
}

func TestMarshalChatRequestBodySkipsThinkingDisabled(t *testing.T) {
	chatReq := toolCallChatRequest(t, "deepseek-v4-flash",
		`, "thinking": {"type": "disabled"}`)

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	_, ok := messageReasoningContent(t, messages[1])
	assert.False(t, ok, "requests with thinking explicitly disabled must not be modified")
}

func TestMarshalChatRequestBodyIgnoresNullToolCalls(t *testing.T) {
	raw := []byte(`{
		"model": "deepseek-v4-flash",
		"messages": [
			{"role": "assistant", "content": "final answer", "tool_calls": null},
			{"role": "user", "content": "thanks"}
		],
		"tools": [{"type": "function", "function": {"name": "get_weather"}}]
	}`)
	chatReq := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal(raw, chatReq))

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 2)
	_, ok := messageReasoningContent(t, messages[0])
	assert.False(t, ok, "assistant messages without actual tool calls must not be modified")
}

func TestMarshalChatRequestBodyRawPassthroughPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{
		"model": "deepseek-v4-flash",
		"messages": [
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": "4", "reasoning_content": "thinking",
			 "future_provider_field": {"preserve": true},
			 "cache_control": {"type": "ephemeral"},
			 "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "add", "arguments": "{}"}, "vendor_meta": {"x": 1}}]}
		],
		"tools": [{"type": "function", "function": {"name": "add", "vendor_option": {"strict": true}}}]
	}`)
	chatReq := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal(raw, chatReq))

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	var payload struct {
		Model    string            `json:"model"`
		Messages []json.RawMessage `json:"messages"`
		Tools    []json.RawMessage `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))

	// vendor-specific message and tool fields must survive the round trip
	assert.Contains(t, string(payload.Messages[1]), `"future_provider_field":{"preserve":true}`)
	assert.Contains(t, string(payload.Messages[1]), `"cache_control":{"type":"ephemeral"}`)
	assert.Contains(t, string(payload.Messages[1]), `"vendor_meta":{"x":1}`)
	assert.Contains(t, string(payload.Tools[0]), `"vendor_option":{"strict":true}`)
	// the gateway-mandated overrides still apply
	assert.Equal(t, "deepseek-flash", payload.Model)
	reasoning, ok := messageReasoningContent(t, payload.Messages[1])
	require.True(t, ok)
	assert.Equal(t, "thinking", reasoning, "client-passed reasoning_content must not be overwritten")
}

func TestMarshalChatRequestBodyRawStreamOptions(t *testing.T) {
	build := func(t *testing.T, raw string) *types.ChatCompletionRequest {
		t.Helper()
		chatReq := &types.ChatCompletionRequest{}
		require.NoError(t, json.Unmarshal([]byte(raw), chatReq))
		return chatReq
	}

	t.Run("gateway-forced streaming gains include_usage", func(t *testing.T) {
		chatReq := build(t, `{"model": "m", "stream": true, "messages": [{"role": "user", "content": "hi"}]}`)
		chatReq.ForceStreamUsage = true
		body, err := marshalChatRequestBody(chatReq, "upstream-m")
		require.NoError(t, err)
		assert.Contains(t, string(body), `"stream_options":{"include_usage":true}`)
	})

	t.Run("gateway-forced streaming overrides client include_usage false", func(t *testing.T) {
		chatReq := build(t, `{"model": "m", "stream": true, "stream_options": {"include_usage": false}, "messages": [{"role": "user", "content": "hi"}]}`)
		chatReq.ForceStreamUsage = true
		body, err := marshalChatRequestBody(chatReq, "upstream-m")
		require.NoError(t, err)
		assert.Contains(t, string(body), `"include_usage":true`)
		assert.NotContains(t, string(body), `"include_usage":false`)
	})

	t.Run("non-streaming client stream_options are forwarded verbatim", func(t *testing.T) {
		// A client may send stream_options on a non-streaming request; the
		// gateway must not inject or alter it (P9 regression).
		chatReq := build(t, `{"model": "m", "stream": false, "stream_options": {"include_usage": true}, "messages": [{"role": "user", "content": "hi"}]}`)
		body, err := marshalChatRequestBody(chatReq, "m")
		require.NoError(t, err)
		assert.Equal(t, string(chatReq.RawBody), string(body), "nothing to override: verbatim fast path")
		assert.Contains(t, string(body), `"include_usage": true`)
	})

	t.Run("non-forced streaming client stream_options are forwarded verbatim", func(t *testing.T) {
		// vllm-cpu targets skip the gateway force; the client's value must
		// survive untouched (P10 regression).
		chatReq := build(t, `{"model": "m", "stream": true, "stream_options": {"include_usage": false}, "messages": [{"role": "user", "content": "hi"}]}`)
		body, err := marshalChatRequestBody(chatReq, "m")
		require.NoError(t, err)
		assert.Equal(t, string(chatReq.RawBody), string(body), "nothing to override: verbatim fast path")
		assert.Contains(t, string(body), `"include_usage": false`)
		assert.NotContains(t, string(body), `"include_usage": true`)
	})

	t.Run("requests without stream_options keep it absent", func(t *testing.T) {
		chatReq := build(t, `{"model": "m", "stream": false, "messages": [{"role": "user", "content": "hi"}]}`)
		body, err := marshalChatRequestBody(chatReq, "m")
		require.NoError(t, err)
		assert.NotContains(t, string(body), "stream_options")
	})

	t.Run("null client stream_options does not panic", func(t *testing.T) {
		chatReq := build(t, `{"model": "m", "stream": true, "stream_options": null, "messages": [{"role": "user", "content": "hi"}]}`)
		chatReq.ForceStreamUsage = true
		body, err := marshalChatRequestBody(chatReq, "upstream-m")
		require.NoError(t, err)
		assert.Contains(t, string(body), `"stream_options":{"include_usage":true}`)
	})
}

func TestMarshalChatRequestBodyStructPathWithoutRawBody(t *testing.T) {
	// Programmatically built requests (protocol adapters) have no raw body
	// and keep the typed serialization path, including the fixup.
	chatReq := toolCallChatRequest(t, "deepseek-v4-flash", "")
	chatReq.RawBody = nil

	body, err := marshalChatRequestBody(chatReq, "deepseek-flash")
	require.NoError(t, err)

	messages := decodeMessages(t, body)
	require.Len(t, messages, 3)
	reasoning, ok := messageReasoningContent(t, messages[1])
	require.True(t, ok, "adapter-built requests must still get the reasoning fixup")
	assert.Empty(t, reasoning)
}

func TestMarshalChatRequestBodyRawFastPath(t *testing.T) {
	raw := []byte(`{"model": "served-model", "stream": false, "messages": [{"role": "user", "content": "hi"}], "vendor_field": {"keep": true}}`)
	chatReq := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal(raw, chatReq))

	t.Run("forwards client bytes untouched when nothing must be overridden", func(t *testing.T) {
		body, err := marshalChatRequestBody(chatReq, "served-model")
		require.NoError(t, err)
		assert.Equal(t, raw, body, "fast path must return the client's bytes verbatim")
	})

	t.Run("handler-side model overwrite does not affect the fast path decision", func(t *testing.T) {
		// Execute overwrites chatReq.Model with the resolved upstream name;
		// ClientModel must keep driving the decision.
		chatReq.Model = "served-model"
		body, err := marshalChatRequestBody(chatReq, "served-model")
		require.NoError(t, err)
		assert.Equal(t, raw, body)
	})

	t.Run("model rewrite forces the patch path", func(t *testing.T) {
		body, err := marshalChatRequestBody(chatReq, "other-upstream-name")
		require.NoError(t, err)
		assert.NotEqual(t, raw, body)
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.JSONEq(t, `"other-upstream-name"`, string(payload["model"]))
		assert.Contains(t, string(body), `"vendor_field":{"keep":true}`,
			"patch path must still forward client fields")
	})

	t.Run("forced usage option forces the patch path", func(t *testing.T) {
		chatReq.ForceStreamUsage = true
		defer func() { chatReq.ForceStreamUsage = false }()
		body, err := marshalChatRequestBody(chatReq, "served-model")
		require.NoError(t, err)
		assert.NotEqual(t, raw, body)
		assert.Contains(t, string(body), `"include_usage":true`)
	})

	t.Run("reasoning fixup forces the patch path", func(t *testing.T) {
		rawWithTools := []byte(`{"model": "deepseek-flash", "messages": [{"role": "assistant", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "f"}}]}], "tools": [{"type": "function", "function": {"name": "f"}}]}`)
		chatReqWithTools := &types.ChatCompletionRequest{}
		require.NoError(t, json.Unmarshal(rawWithTools, chatReqWithTools))
		body, err := marshalChatRequestBody(chatReqWithTools, "deepseek-flash")
		require.NoError(t, err)
		assert.Contains(t, string(body), `"reasoning_content":""`)
	})
}
