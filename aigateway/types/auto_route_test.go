package types

import (
	"encoding/json"
	"net/http"
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
		assert.Empty(t, meta.AutoRouteContext().CurrentInput)
		assert.Empty(t, meta.AutoRouteContext().History)
		assert.Empty(t, (&RequestMetadata{}).AutoRouteContext().CurrentInput)
	})

	t.Run("a body that cannot describe itself", func(t *testing.T) {
		meta := &RequestMetadata{ParsedBody: struct{}{}}
		assert.Empty(t, meta.AutoRouteContext().CurrentInput)
	})

	t.Run("delegates to the body", func(t *testing.T) {
		meta := &RequestMetadata{ParsedBody: &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
		}}
		input := meta.AutoRouteContext()
		require.Len(t, input.CurrentInput, 1)
		assert.Equal(t, "hi", input.CurrentInput[0].Content)
		assert.Empty(t, input.History)
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
	assert.Equal(t, "You are a coding agent.", input.SystemPrompt)
	require.Len(t, input.CurrentInput, 1)
	assert.Equal(t, "user", input.CurrentInput[0].Role)
	assert.Equal(t, "Fix the failing test in parser.go", input.CurrentInput[0].Content)
	require.Len(t, input.History, 1)
	assert.Equal(t, "assistant", input.History[0].Role)
	assert.Equal(t, "Looking now.", input.History[0].Content)
	require.Len(t, input.Tools, 1)
	assert.Contains(t, string(input.Tools[0]), "read_file")
	// The routing identity is header-sourced, not a body field, so the DTO
	// itself carries none of it.
	assert.Empty(t, input.SessionID)
	assert.Empty(t, input.TurnID)

	var nilReq *ChatCompletionRequest
	assert.Empty(t, nilReq.AutoRouteContext().CurrentInput)
}

func TestChatCompletionRequest_AutoRouteContext_MultipartContent(t *testing.T) {
	req := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"text","text":"this"}]}]
	}`), req))

	input := req.AutoRouteContext()
	require.Len(t, input.CurrentInput, 1)
	assert.Equal(t, "user", input.CurrentInput[0].Role)
	// Multipart content arrives as the SDK's typed slice rather than a
	// []interface{}, so it is serialized whole.  Both text parts are still
	// present, which is what the ranking service needs.
	assert.Contains(t, input.CurrentInput[0].Content, "describe")
	assert.Contains(t, input.CurrentInput[0].Content, "this")
	assert.Empty(t, input.History)
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
	require.Len(t, input.CurrentInput, 1)
	assert.Equal(t, "be brief", input.SystemPrompt)

	prompt := req.PromptText()
	for _, message := range append(append([]AutoRouteMessage{}, input.History...), input.CurrentInput...) {
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
		assert.Equal(t, "You are terse.", input.SystemPrompt)
		require.Len(t, input.CurrentInput, 2)
		assert.Equal(t, "user", input.CurrentInput[0].Role)
		assert.Contains(t, input.CurrentInput[0].Content, "what is 2+2")
		assert.Equal(t, "tool", input.CurrentInput[1].Role, "the function output is part of the current turn")
		require.Len(t, input.History, 1)
		assert.Equal(t, "assistant", input.History[0].Role, "a function call is attributed to the assistant")
		require.Len(t, input.Tools, 1)
		assert.Empty(t, input.SessionID, "routing identity is header-sourced")
		assert.Empty(t, input.TurnID)
	})

	t.Run("bare string input", func(t *testing.T) {
		req := &ResponsesRequest{Input: json.RawMessage(`"hello there"`)}
		input := req.AutoRouteContext()
		require.Len(t, input.CurrentInput, 1)
		assert.Equal(t, "user", input.CurrentInput[0].Role)
		assert.Equal(t, "hello there", input.CurrentInput[0].Content)
		assert.Empty(t, input.History)
	})

	t.Run("nil request", func(t *testing.T) {
		var req *ResponsesRequest
		assert.Empty(t, req.AutoRouteContext().CurrentInput)
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
	assert.Equal(t, "You are a coding agent.", input.SystemPrompt)
	require.Len(t, input.CurrentInput, 1)
	assert.Equal(t, "user", input.CurrentInput[0].Role)
	assert.Equal(t, "fix parser.go", input.CurrentInput[0].Content)
	require.Len(t, input.History, 1)
	assert.Equal(t, "assistant", input.History[0].Role)
	assert.Equal(t, "on it", input.History[0].Content)
	require.Len(t, input.Tools, 1)
	assert.Contains(t, string(input.Tools[0]), "read_file")
	assert.Empty(t, input.SessionID, "routing identity is header-sourced")
	assert.Empty(t, input.TurnID)

	var nilReq *AnthropicMessagesRequest
	assert.Empty(t, nilReq.AutoRouteContext().CurrentInput)
}

func TestSplitAutoRouteMessages(t *testing.T) {
	systemPrompt, history, current := splitAutoRouteMessages([]AutoRouteMessage{
		{Role: "system", Content: "be brief"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
		{Role: "tool", Content: "tool output"},
	})

	assert.Equal(t, "be brief", systemPrompt)
	assert.Equal(t, []AutoRouteMessage{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
	}, history)
	assert.Equal(t, []AutoRouteMessage{
		{Role: "user", Content: "second question"},
		{Role: "tool", Content: "tool output"},
	}, current)
}

func TestSplitAutoRouteMessages_TrailingEmptyUser(t *testing.T) {
	systemPrompt, history, current := splitAutoRouteMessages([]AutoRouteMessage{
		{Role: "user", Content: "q"},
		{Role: "user", Content: ""},
	})

	assert.Empty(t, systemPrompt)
	assert.Empty(t, history)
	assert.Equal(t, []AutoRouteMessage{
		{Role: "user", Content: "q"},
	}, current)
}

func TestRequestMetadata_AutoRouteContext_RoutingIdentity(t *testing.T) {
	body := &ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"auto",
		"messages":[{"role":"user","content":"hi"}]
	}`), body))

	headers := func() http.Header { return http.Header{} }

	t.Run("explicit router headers", func(t *testing.T) {
		h := headers()
		h.Set(HeaderRouterSessionID, "ses_0123456789abcdef0123456789abcdef")
		h.Set(HeaderRouterTurnID, "turn_0123456789abcdef0123456789abcdef")
		meta := &RequestMetadata{ParsedBody: body, Headers: h}
		input := meta.AutoRouteContext()
		assert.Equal(t, "ses_0123456789abcdef0123456789abcdef", input.SessionID)
		assert.Equal(t, "turn_0123456789abcdef0123456789abcdef", input.TurnID)
		require.Len(t, input.CurrentInput, 1)
	})

	t.Run("X-Session-ID alias for session only", func(t *testing.T) {
		h := headers()
		h.Set(HeaderSessionAlias, "ses_0123456789abcdef0123456789abcdef")
		meta := &RequestMetadata{ParsedBody: body, Headers: h}
		input := meta.AutoRouteContext()
		assert.Equal(t, "ses_0123456789abcdef0123456789abcdef", input.SessionID)
		assert.Empty(t, input.TurnID, "the alias carries session only, never turn")
	})

	t.Run("explicit router session wins over X-Session-ID alias", func(t *testing.T) {
		h := headers()
		h.Set(HeaderRouterSessionID, "ses_aabbccddeeff00112233445566778899")
		h.Set(HeaderSessionAlias, "ses_0123456789abcdef0123456789abcdef")
		h.Set(HeaderRouterTurnID, "turn_aabbccddeeff00112233445566778899")
		meta := &RequestMetadata{ParsedBody: body, Headers: h}
		input := meta.AutoRouteContext()
		assert.Equal(t, "ses_aabbccddeeff00112233445566778899", input.SessionID)
		assert.Equal(t, "turn_aabbccddeeff00112233445566778899", input.TurnID)
	})

	t.Run("invalid-format identity is dropped", func(t *testing.T) {
		h := headers()
		h.Set(HeaderRouterSessionID, "not-a-valid-session")
		h.Set(HeaderSessionAlias, "my-trace-id")
		h.Set(HeaderRouterTurnID, "not-a-valid-turn")
		meta := &RequestMetadata{ParsedBody: body, Headers: h}
		input := meta.AutoRouteContext()
		assert.Empty(t, input.SessionID, "arbitrary X-Session-ID values are not forwarded as router session IDs")
		assert.Empty(t, input.TurnID)
		require.Len(t, input.CurrentInput, 1, "the turn body is still described")
	})

	t.Run("turn without session is dropped", func(t *testing.T) {
		h := headers()
		h.Set(HeaderRouterTurnID, "turn_0123456789abcdef0123456789abcdef")
		meta := &RequestMetadata{ParsedBody: body, Headers: h}
		input := meta.AutoRouteContext()
		assert.Empty(t, input.SessionID)
		assert.Empty(t, input.TurnID, "the router requires a session for every turn")
		require.Len(t, input.CurrentInput, 1, "the request falls back to a new router session")
	})

	t.Run("no identity headers", func(t *testing.T) {
		meta := &RequestMetadata{ParsedBody: body, Headers: headers()}
		input := meta.AutoRouteContext()
		assert.Empty(t, input.SessionID)
		assert.Empty(t, input.TurnID)
		require.Len(t, input.CurrentInput, 1, "the turn body is still described")
	})
}
