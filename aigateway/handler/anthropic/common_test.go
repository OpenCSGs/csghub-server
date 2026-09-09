package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestCheckAdapterCapabilities(t *testing.T) {
	t.Run("all satisfied", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		cap := types.CapabilityFor(types.ProtocolChat)
		missing := checkAdapterCapabilities(req, cap)
		assert.Empty(t, missing)
	})

	t.Run("cache_control not supported", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]`)},
			},
		}
		cap := types.CapabilityFor(types.ProtocolChat)
		missing := checkAdapterCapabilities(req, cap)
		assert.Contains(t, missing, "prompt_caching")
	})

t.Run("thinking not supported", func(t *testing.T) {
			req := &types.AnthropicMessagesRequest{
				Thinking: &types.AnthropicThinking{Type: "enabled", BudgetTokens: 5000},
				Messages: []types.AnthropicMessage{
					{Role: "user", Content: json.RawMessage(`"hello"`)},
				},
			}
			// Use a custom capability with Thinking disabled — the default
			// ProtocolChat capability now supports thinking via reasoning_effort.
			cap := types.ProtocolCapability{Thinking: false}
			missing := checkAdapterCapabilities(req, cap)
			assert.Contains(t, missing, "thinking")
		})

	t.Run("vision not supported", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]`)},
			},
		}
		cap := types.ProtocolCapability{Tools: true}
		missing := checkAdapterCapabilities(req, cap)
		assert.Contains(t, missing, "vision")
	})
}

func TestMessagesToChatMessages(t *testing.T) {
	t.Run("simple text conversation", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			System: json.RawMessage(`"You are helpful"`),
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"Hello"`)},
				{Role: "assistant", Content: json.RawMessage(`"Hi there"`)},
				{Role: "user", Content: json.RawMessage(`"How are you?"`)},
			},
		}
		msgs, err := messagesToChatMessages(req, true)
		require.NoError(t, err)
		require.Len(t, msgs, 4)

		assert.Equal(t, "system", msgs[0]["role"])
		assert.Equal(t, "You are helpful", msgs[0]["content"])

		assert.Equal(t, "user", msgs[1]["role"])
		assert.Equal(t, "Hello", msgs[1]["content"])

		assert.Equal(t, "assistant", msgs[2]["role"])
		assert.Equal(t, "Hi there", msgs[2]["content"])

		assert.Equal(t, "user", msgs[3]["role"])
		assert.Equal(t, "How are you?", msgs[3]["content"])
	})

	t.Run("tool use and tool result", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{
					Role: "user",
					Content: json.RawMessage(`"What's the weather?"`),
				},
				{
					Role: "assistant",
					Content: json.RawMessage(`[{"type":"tool_use","id":"tool_1","name":"get_weather","input":{"city":"SF"}}]`),
				},
				{
					Role: "user",
					Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"tool_1","content":"72°F sunny"}]`),
				},
			},
		}
		msgs, err := messagesToChatMessages(req, true)
		require.NoError(t, err)
		require.Len(t, msgs, 3)

		// User message
		assert.Equal(t, "user", msgs[0]["role"])

		// Assistant with tool_calls
		assert.Equal(t, "assistant", msgs[1]["role"])
		toolCalls, ok := msgs[1]["tool_calls"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, toolCalls, 1)
		assert.Equal(t, "tool_1", toolCalls[0]["id"])
		fn := toolCalls[0]["function"].(map[string]any)
		assert.Equal(t, "get_weather", fn["name"])

		// Tool result
		assert.Equal(t, "tool", msgs[2]["role"])
		assert.Equal(t, "tool_1", msgs[2]["tool_call_id"])
		assert.Equal(t, "72°F sunny", msgs[2]["content"])
	})

	t.Run("thinking blocks", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{
					Role: "assistant",
					Content: json.RawMessage(`[{"type":"thinking","thinking":"Let me think..."},{"type":"text","text":"The answer is 42"}]`),
				},
			},
		}
		msgs, err := messagesToChatMessages(req, true)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "assistant", msgs[0]["role"])
		assert.Equal(t, "The answer is 42", msgs[0]["content"])
		assert.Equal(t, "Let me think...", msgs[0]["reasoning_content"])
	})

	t.Run("thinking blocks without reasoning support", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Messages: []types.AnthropicMessage{
				{
					Role:    "assistant",
					Content: json.RawMessage(`[{"type":"thinking","thinking":"Let me think..."},{"type":"text","text":"The answer is 42"}]`),
				},
			},
		}
		// When the upstream does not support thinking, reasoning_content should be omitted.
		msgs, err := messagesToChatMessages(req, false)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "assistant", msgs[0]["role"])
		assert.Equal(t, "The answer is 42", msgs[0]["content"])
		_, hasReasoning := msgs[0]["reasoning_content"]
		assert.False(t, hasReasoning)
	})

	t.Run("system as content blocks", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			System: json.RawMessage(`[{"type":"text","text":"Be concise"}]`),
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"Hello"`)},
			},
		}
		msgs, err := messagesToChatMessages(req, true)
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, "system", msgs[0]["role"])
		assert.Equal(t, "Be concise", msgs[0]["content"])
	})
}

func TestMessagesToolsToChatTools(t *testing.T) {
	tools := []types.AnthropicTool{
		{
			Name:        "get_weather",
			Description: "Get the weather",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		},
	}
	chatTools := messagesToolsToChatTools(tools)
	require.Len(t, chatTools, 1)
	assert.Equal(t, "function", chatTools[0]["type"])
	fn := chatTools[0]["function"].(map[string]any)
	assert.Equal(t, "get_weather", fn["name"])
	assert.Equal(t, "Get the weather", fn["description"])
}

func TestChatResponseToMessagesResponse(t *testing.T) {
	t.Run("text response", func(t *testing.T) {
		chatJSON := `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "qwen3",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`
		resp, err := chatResponseToMessagesResponse([]byte(chatJSON), "claude-3")
		require.NoError(t, err)
		assert.Equal(t, "message", resp.Type)
		assert.Equal(t, "assistant", resp.Role)
		require.Len(t, resp.Content, 1)
		assert.Equal(t, "text", resp.Content[0].Type)
		assert.Equal(t, "Hello!", resp.Content[0].Text)
		assert.Equal(t, "end_turn", *resp.StopReason)
		assert.Equal(t, 10, resp.Usage.InputTokens)
		assert.Equal(t, 5, resp.Usage.OutputTokens)
	})

	t.Run("tool use response", func(t *testing.T) {
		chatJSON := `{
			"id": "chatcmpl-2",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "qwen3",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`
		resp, err := chatResponseToMessagesResponse([]byte(chatJSON), "claude-3")
		require.NoError(t, err)
		require.Len(t, resp.Content, 1)
		assert.Equal(t, "tool_use", resp.Content[0].Type)
		assert.Equal(t, "call_1", resp.Content[0].ID)
		assert.Equal(t, "get_weather", resp.Content[0].Name)
		assert.Equal(t, "tool_use", *resp.StopReason)
	})

	t.Run("with reasoning", func(t *testing.T) {
		chatJSON := `{
			"id": "chatcmpl-3",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "qwen3",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "42", "reasoning_content": "Thinking..."},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`
		resp, err := chatResponseToMessagesResponse([]byte(chatJSON), "claude-3")
		require.NoError(t, err)
		require.Len(t, resp.Content, 2)
		assert.Equal(t, "thinking", resp.Content[0].Type)
		assert.Equal(t, "Thinking...", resp.Content[0].Thinking)
		assert.Equal(t, "text", resp.Content[1].Type)
		assert.Equal(t, "42", resp.Content[1].Text)
	})

	t.Run("max_tokens finish reason", func(t *testing.T) {
		chatJSON := `{
			"id": "chatcmpl-4",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "qwen3",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "partial..."},
				"finish_reason": "length"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 100, "total_tokens": 110}
		}`
		resp, err := chatResponseToMessagesResponse([]byte(chatJSON), "claude-3")
		require.NoError(t, err)
		assert.Equal(t, "max_tokens", *resp.StopReason)
	})
}

func TestResponsesResponseToMessagesResponse(t *testing.T) {
	t.Run("text output", func(t *testing.T) {
		resp := &types.ResponsesResponse{
			ID:     "resp_1",
			Status: "completed",
			Model:  "gpt-4",
			Output: []types.ResponsesOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []types.ResponsesContentPart{
						{Type: "output_text", Text: "Hello from responses!"},
					},
				},
			},
			Usage: &types.ResponsesUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}
		result, err := responsesResponseToMessagesResponse(resp, "claude-3")
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		assert.Equal(t, "text", result.Content[0].Type)
		assert.Equal(t, "Hello from responses!", result.Content[0].Text)
		assert.Equal(t, "end_turn", *result.StopReason)
		assert.Equal(t, 10, result.Usage.InputTokens)
	})

	t.Run("function call output", func(t *testing.T) {
		resp := &types.ResponsesResponse{
			Status: "completed",
			Output: []types.ResponsesOutputItem{
				{
					Type:      "function_call",
					CallID:    "call_1",
					Name:      "get_weather",
					Arguments: `{"city":"SF"}`,
				},
			},
		}
		result, err := responsesResponseToMessagesResponse(resp, "claude-3")
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		assert.Equal(t, "tool_use", result.Content[0].Type)
		assert.Equal(t, "call_1", result.Content[0].ID)
	})

	t.Run("reasoning output", func(t *testing.T) {
		resp := &types.ResponsesResponse{
			Status: "completed",
			Output: []types.ResponsesOutputItem{
				{
					Type: "reasoning",
					Summary: []types.ResponsesSummaryPart{
						{Type: "summary_text", Text: "Reasoning here"},
					},
				},
			},
		}
		result, err := responsesResponseToMessagesResponse(resp, "claude-3")
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		assert.Equal(t, "thinking", result.Content[0].Type)
		assert.Equal(t, "Reasoning here", result.Content[0].Thinking)
	})

	t.Run("incomplete status", func(t *testing.T) {
		resp := &types.ResponsesResponse{
			Status: "incomplete",
			Output: []types.ResponsesOutputItem{
				{Type: "message", Role: "assistant", Content: []types.ResponsesContentPart{{Type: "output_text", Text: "partial"}}},
			},
		}
		result, err := responsesResponseToMessagesResponse(resp, "claude-3")
		require.NoError(t, err)
		assert.Equal(t, "max_tokens", *result.StopReason)
	})
}

func TestMessagesToResponsesRequest(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		temp := 0.7
		req := &types.AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
			System:    json.RawMessage(`"Be helpful"`),
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"Hello"`)},
				{Role: "assistant", Content: json.RawMessage(`"Hi"`)},
				{Role: "user", Content: json.RawMessage(`"How are you?"`)},
			},
			Temperature: &temp,
		}
		respReq, err := messagesToResponsesRequest(req, "claude-3")
		require.NoError(t, err)
		assert.Equal(t, "claude-3", respReq.Model)
		assert.NotNil(t, respReq.MaxOutputTokens)
		assert.Equal(t, 1024, *respReq.MaxOutputTokens)
		assert.NotNil(t, respReq.Temperature)
		assert.Equal(t, 0.7, *respReq.Temperature)

		// Check instructions
		var instrStr string
		require.NoError(t, json.Unmarshal(respReq.Instructions, &instrStr))
		assert.Equal(t, "Be helpful", instrStr)

		// Check input items
		var items []map[string]any
		require.NoError(t, json.Unmarshal(respReq.Input, &items))
		require.Len(t, items, 3)
	})

	t.Run("with tools", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"What's the weather?"`)},
			},
			Tools: []types.AnthropicTool{
				{Name: "get_weather", Description: "Get weather", InputSchema: json.RawMessage(`{"type":"object"}`)},
			},
		}
		respReq, err := messagesToResponsesRequest(req, "claude-3")
		require.NoError(t, err)
		require.NotNil(t, respReq.Tools)

		var tools []map[string]any
		require.NoError(t, json.Unmarshal(respReq.Tools, &tools))
		require.Len(t, tools, 1)
		assert.Equal(t, "function", tools[0]["type"])
		assert.Equal(t, "get_weather", tools[0]["name"])
	})

	t.Run("with thinking", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"Think carefully"`)},
			},
			Thinking: &types.AnthropicThinking{Type: "enabled", BudgetTokens: 10000},
		}
		respReq, err := messagesToResponsesRequest(req, "claude-3")
		require.NoError(t, err)
		require.NotNil(t, respReq.Reasoning)

		var reasoning map[string]any
		require.NoError(t, json.Unmarshal(respReq.Reasoning, &reasoning))
		assert.Equal(t, "medium", reasoning["effort"])
	})

	t.Run("tool use and result in messages", func(t *testing.T) {
		req := &types.AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
			Messages: []types.AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"Weather?"`)},
				{
					Role:    "assistant",
					Content: json.RawMessage(`[{"type":"tool_use","id":"tool_1","name":"get_weather","input":{"city":"SF"}}]`),
				},
				{
					Role:    "user",
					Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"tool_1","content":"72F"}]`),
				},
			},
		}
		respReq, err := messagesToResponsesRequest(req, "claude-3")
		require.NoError(t, err)

		var items []map[string]any
		require.NoError(t, json.Unmarshal(respReq.Input, &items))
		require.Len(t, items, 3)

		// The third item should be function_call_output
		assert.Equal(t, "function_call_output", items[2]["type"])
		assert.Equal(t, "tool_1", items[2]["call_id"])
	})
}

func TestChatFinishReasonToStopReason(t *testing.T) {
	tests := []struct {
		finishReason string
		expected     string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"function_call", "tool_use"},
		{"stop_sequence", "stop_sequence"},
		{"unknown", "end_turn"},
	}
	for _, tt := range tests {
		t.Run(tt.finishReason, func(t *testing.T) {
			assert.Equal(t, tt.expected, chatFinishReasonToStopReason(tt.finishReason))
		})
	}
}

func TestBudgetToEffort(t *testing.T) {
	tests := []struct {
		budget   int
		expected string
	}{
		{1000, "low"},
		{5000, "low"},
		{10000, "medium"},
		{20000, "medium"},
		{50000, "high"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, budgetToEffort(tt.budget))
		})
	}
}

func TestNewMessagesResponseID(t *testing.T) {
	id1 := newMessagesResponseID()
	id2 := newMessagesResponseID()
	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "msg_agw_")
}

func TestConvertToolChoiceForChat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"auto", `{"type":"auto"}`, `"auto"`},
		{"any → required", `{"type":"any"}`, `"required"`},
		{"tool → function", `{"type":"tool","name":"get_weather"}`, `{"type":"function","function":{"name":"get_weather"}}`},
		{"unknown defaults to auto", `{"type":"weird"}`, `"auto"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToolChoiceForChat(json.RawMessage(tt.input))
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(result))
		})
	}
}

func TestConvertToolChoiceForResponses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"auto", `{"type":"auto"}`, `"auto"`},
		{"any → required", `{"type":"any"}`, `"required"`},
		{"tool → function (flat)", `{"type":"tool","name":"get_weather"}`, `{"type":"function","name":"get_weather"}`},
		{"unknown defaults to auto", `{"type":"weird"}`, `"auto"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToolChoiceForResponses(json.RawMessage(tt.input))
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(result))
		})
	}
}
