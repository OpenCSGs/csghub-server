package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAutoRouteRole(t *testing.T) {
	for _, role := range []string{"system", "developer", "user", "assistant", "tool"} {
		assert.Equal(t, role, normalizeAutoRouteRole(role))
	}
	assert.Equal(t, "user", normalizeAutoRouteRole("function"), "an unknown role still contributes its text")
	assert.Equal(t, "user", normalizeAutoRouteRole(""))
}

func TestRequestMetadata_AutoRouteContext(t *testing.T) {
	t.Run("nil metadata and body", func(t *testing.T) {
		var meta *RequestMetadata
		assert.Empty(t, meta.AutoRouteContext().Messages)
		assert.Empty(t, (&RequestMetadata{}).AutoRouteContext().Messages)
	})

	t.Run("a body that cannot describe itself", func(t *testing.T) {
		meta := &RequestMetadata{ParsedBody: struct{}{}}
		assert.Empty(t, meta.AutoRouteContext().Messages)
	})

	t.Run("delegates to the body", func(t *testing.T) {
		meta := &RequestMetadata{ParsedBody: &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
		}}
		input := meta.AutoRouteContext()
		require.Len(t, input.Messages, 1)
		assert.Equal(t, "hi", input.Messages[0].Content)
	})
}

func TestChatCompletionRequest_AutoRouteContext(t *testing.T) {
	req := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"auto",
		"messages":[
			{"role":"system","content":"You are a coding agent."},
			{"role":"user","content":"Fix the failing test in parser.go"},
			{"role":"assistant","content":"Looking now."}
		],
		"tools":[{"type":"function","function":{"name":"read_file","parameters":{"type":"object","properties":{}}}}]
	}`), req))

	input := req.AutoRouteContext()
	require.Len(t, input.Messages, 3)
	assert.Equal(t, "system", input.Messages[0].Role)
	assert.Equal(t, "You are a coding agent.", input.Messages[0].Content)
	assert.Equal(t, "user", input.Messages[1].Role)
	assert.Equal(t, "Fix the failing test in parser.go", input.Messages[1].Content)
	assert.Equal(t, "assistant", input.Messages[2].Role)
	require.Len(t, input.Tools, 1)
	assert.Contains(t, string(input.Tools[0]), "read_file")

	var nilReq *ChatCompletionRequest
	assert.Empty(t, nilReq.AutoRouteContext().Messages)
}

func TestChatCompletionRequest_AutoRouteContext_MultipartContent(t *testing.T) {
	req := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"text","text":"this"}]}]
	}`), req))

	input := req.AutoRouteContext()
	require.Len(t, input.Messages, 1)
	assert.Equal(t, "user", input.Messages[0].Role)
	// Multipart content arrives as the SDK's typed slice rather than a
	// []interface{}, so it is serialized whole.  Both text parts are still
	// present, which is what the ranking service needs.
	assert.Contains(t, input.Messages[0].Content, "describe")
	assert.Contains(t, input.Messages[0].Content, "this")
}

// PromptText and AutoRouteContext share the per-message extraction, so
// both must see the same text for the same message.
func TestChatMessageText_SharedByPromptTextAndAutoRoute(t *testing.T) {
	req := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"messages":[
			{"role":"system","content":"be brief"},
			{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}
		]
	}`), req))

	input := req.AutoRouteContext()
	require.Len(t, input.Messages, 2)

	prompt := req.PromptText()
	for _, message := range input.Messages {
		assert.Contains(t, prompt, message.Content)
	}
}

func TestResponsesRequest_AutoRouteContext(t *testing.T) {
	t.Run("instructions plus structured input", func(t *testing.T) {
		req := &ResponsesRequest{
			Instructions: json.RawMessage(`"You are terse."`),
			Input: json.RawMessage(`[
				{"role":"user","content":[{"type":"input_text","text":"what is 2+2"}]},
				{"type":"function_call","name":"calc","arguments":"{\"a\":2}"},
				{"type":"function_call_output","output":"4"}
			]`),
			Tools: json.RawMessage(`[{"type":"function","name":"calc"}]`),
		}

		input := req.AutoRouteContext()
		require.Len(t, input.Messages, 4)
		assert.Equal(t, "system", input.Messages[0].Role)
		assert.Equal(t, "You are terse.", input.Messages[0].Content)
		assert.Equal(t, "user", input.Messages[1].Role)
		assert.Contains(t, input.Messages[1].Content, "what is 2+2")
		assert.Equal(t, "assistant", input.Messages[2].Role, "a function call is attributed to the assistant")
		assert.Equal(t, "tool", input.Messages[3].Role, "its output is attributed to the tool")
		require.Len(t, input.Tools, 1)
	})

	t.Run("bare string input", func(t *testing.T) {
		req := &ResponsesRequest{Input: json.RawMessage(`"hello there"`)}
		input := req.AutoRouteContext()
		require.Len(t, input.Messages, 1)
		assert.Equal(t, "user", input.Messages[0].Role)
		assert.Equal(t, "hello there", input.Messages[0].Content)
	})

	t.Run("nil request", func(t *testing.T) {
		var req *ResponsesRequest
		assert.Empty(t, req.AutoRouteContext().Messages)
	})
}

func TestAnthropicMessagesRequest_AutoRouteContext(t *testing.T) {
	req := &AnthropicMessagesRequest{
		System: json.RawMessage(`"You are a coding agent."`),
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"fix parser.go"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"on it"}]`)},
		},
		Tools: []AnthropicTool{{Name: "read_file"}},
	}

	input := req.AutoRouteContext()
	require.Len(t, input.Messages, 3)
	assert.Equal(t, "system", input.Messages[0].Role)
	assert.Equal(t, "You are a coding agent.", input.Messages[0].Content)
	assert.Equal(t, "user", input.Messages[1].Role)
	assert.Equal(t, "fix parser.go", input.Messages[1].Content)
	assert.Equal(t, "assistant", input.Messages[2].Role)
	assert.Equal(t, "on it", input.Messages[2].Content)
	require.Len(t, input.Tools, 1)
	assert.Contains(t, string(input.Tools[0]), "read_file")

	var nilReq *AnthropicMessagesRequest
	assert.Empty(t, nilReq.AutoRouteContext().Messages)
}
