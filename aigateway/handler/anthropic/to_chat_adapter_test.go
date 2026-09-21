package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalToChatRequestBodyInjectsToolCallReasoningContent(t *testing.T) {
	chatReq := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{
				"role":    "assistant",
				"content": "Checking.",
				"tool_calls": []any{
					map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "get_weather"}},
				},
			},
		},
		"tools": []any{map[string]any{"type": "function"}},
	}

	body, err := marshalToChatRequestBody(chatReq, "deepseek-flash", true, false)
	require.NoError(t, err)

	var payload struct {
		Messages []struct {
			Role             string  `json:"role"`
			ReasoningContent *string `json:"reasoning_content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload.Messages, 2)
	require.NotNil(t, payload.Messages[1].ReasoningContent, "assistant tool-call message must carry reasoning_content")
	assert.Empty(t, *payload.Messages[1].ReasoningContent)
	assert.Nil(t, payload.Messages[0].ReasoningContent)
}

func TestMarshalToChatRequestBodySkipsThinkingDisabled(t *testing.T) {
	chatReq := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "assistant", "content": "Checking.", "tool_calls": []any{map[string]any{"id": "call_1"}}},
		},
	}

	body, err := marshalToChatRequestBody(chatReq, "deepseek-flash", true, true)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "reasoning_content")
}

func TestMarshalToChatRequestBodySkipsNonDeepSeekModels(t *testing.T) {
	chatReq := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "assistant", "content": "Checking.", "tool_calls": []any{map[string]any{"id": "call_1"}}},
		},
	}

	body, err := marshalToChatRequestBody(chatReq, "gpt-4o", true, false)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "reasoning_content")
}

func TestMarshalToChatRequestBodySkipsWithoutTools(t *testing.T) {
	chatReq := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "assistant", "content": "Checking.", "tool_calls": []any{map[string]any{"id": "call_1"}}},
		},
	}

	body, err := marshalToChatRequestBody(chatReq, "deepseek-flash", false, false)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "reasoning_content")
}
