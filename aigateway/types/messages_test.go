package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicMessagesRequestValidate(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("missing model", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("missing max_tokens", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Model: "claude-3",
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_tokens")
	})

	t.Run("empty messages", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Model:     "claude-3",
			MaxTokens: 1024,
		}
		err := req.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "messages")
	})
}

func TestAnthropicMessagesRequestUnmarshalJSON(t *testing.T) {
	raw := `{
		"model": "claude-3",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "hello"}],
		"temperature": 0.7,
		"stream": true,
		"custom_field": "custom_value"
	}`

	var req AnthropicMessagesRequest
	err := json.Unmarshal([]byte(raw), &req)
	require.NoError(t, err)

	assert.Equal(t, "claude-3", req.Model)
	assert.Equal(t, 1024, req.MaxTokens)
	assert.True(t, req.Stream)
	assert.NotNil(t, req.Temperature)
	assert.Equal(t, 0.7, *req.Temperature)
	assert.Len(t, req.Messages, 1)

	// Unknown fields preserved
	require.Contains(t, req.ExtraFields, "custom_field")
	var customVal string
	require.NoError(t, json.Unmarshal(req.ExtraFields["custom_field"], &customVal))
	assert.Equal(t, "custom_value", customVal)
}

func TestAnthropicMessagesRequestMarshalJSON(t *testing.T) {
	temp := 0.7
	req := AnthropicMessagesRequest{
		Model:     "claude-3",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Temperature: &temp,
		Stream:      true,
		ExtraFields: map[string]json.RawMessage{
			"custom_field": json.RawMessage(`"custom_value"`),
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "claude-3", m["model"])
	assert.Equal(t, "custom_value", m["custom_field"])
	assert.Equal(t, true, m["stream"])
}

func TestParseAnthropicContentBlocks(t *testing.T) {
	t.Run("string content", func(t *testing.T) {
		blocks, err := ParseAnthropicContentBlocks(json.RawMessage(`"hello world"`))
		require.NoError(t, err)
		require.Len(t, blocks, 1)
		assert.Equal(t, "text", blocks[0].Type)
		assert.Equal(t, "hello world", blocks[0].Text)
	})

	t.Run("array of blocks", func(t *testing.T) {
		raw := `[{"type":"text","text":"hello"},{"type":"text","text":"world"}]`
		blocks, err := ParseAnthropicContentBlocks(json.RawMessage(raw))
		require.NoError(t, err)
		require.Len(t, blocks, 2)
		assert.Equal(t, "hello", blocks[0].Text)
		assert.Equal(t, "world", blocks[1].Text)
	})

	t.Run("null content", func(t *testing.T) {
		blocks, err := ParseAnthropicContentBlocks(json.RawMessage(`null`))
		require.NoError(t, err)
		assert.Nil(t, blocks)
	})

	t.Run("empty content", func(t *testing.T) {
		blocks, err := ParseAnthropicContentBlocks(json.RawMessage(``))
		require.NoError(t, err)
		assert.Nil(t, blocks)
	})

	t.Run("invalid content", func(t *testing.T) {
		_, err := ParseAnthropicContentBlocks(json.RawMessage(`123`))
		assert.Error(t, err)
	})
}

func TestAnthropicMessageContentText(t *testing.T) {
	t.Run("string content", func(t *testing.T) {
		text := AnthropicMessageContentText(json.RawMessage(`"hello"`))
		assert.Equal(t, "hello", text)
	})

	t.Run("array content", func(t *testing.T) {
		raw := `[{"type":"text","text":"hello"},{"type":"text","text":"world"}]`
		text := AnthropicMessageContentText(json.RawMessage(raw))
		assert.Equal(t, "hello\nworld", text)
	})

	t.Run("mixed content types", func(t *testing.T) {
		raw := `[{"type":"image","source":{"type":"base64"}},{"type":"text","text":"describe this"}]`
		text := AnthropicMessageContentText(json.RawMessage(raw))
		assert.Equal(t, "describe this", text)
	})
}

func TestAnthropicMessagesRequestHasCacheControl(t *testing.T) {
	t.Run("with cache_control on content block", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: json.RawMessage(`[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}]`),
				},
			},
		}
		assert.True(t, req.HasCacheControl())
	})

	t.Run("without cache_control", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		assert.False(t, req.HasCacheControl())
	})

	t.Run("with cache_control on tool", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
			Tools: []AnthropicTool{
				{Name: "get_weather", InputSchema: json.RawMessage(`{}`), CacheControl: &AnthropicCacheControl{Type: "ephemeral"}},
			},
		}
		assert.True(t, req.HasCacheControl())
	})
}

func TestAnthropicMessagesRequestHasVision(t *testing.T) {
	t.Run("with image", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: json.RawMessage(`[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR..."}}]`),
				},
			},
		}
		assert.True(t, req.HasVision())
	})

	t.Run("without image", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		assert.False(t, req.HasVision())
	})
}

func TestAnthropicMessagesRequestHasThinking(t *testing.T) {
	t.Run("thinking enabled", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Thinking: &AnthropicThinking{Type: "enabled", BudgetTokens: 5000},
		}
		assert.True(t, req.HasThinking())
	})

	t.Run("thinking adaptive (Claude Code)", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Thinking: &AnthropicThinking{Type: "adaptive"},
		}
		assert.True(t, req.HasThinking())
	})

	t.Run("thinking disabled", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			Thinking: &AnthropicThinking{Type: "disabled"},
		}
		assert.False(t, req.HasThinking())
	})

	t.Run("thinking nil", func(t *testing.T) {
		req := &AnthropicMessagesRequest{}
		assert.False(t, req.HasThinking())
	})
}

func TestAnthropicMessagesRequestPromptText(t *testing.T) {
	t.Run("system string + messages", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			System: json.RawMessage(`"You are a helpful assistant"`),
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"What is 2+2?"`)},
				{Role: "assistant", Content: json.RawMessage(`"4"`)},
			},
		}
		text := req.PromptText()
		assert.Contains(t, text, "You are a helpful assistant")
		assert.Contains(t, text, "What is 2+2?")
		assert.Contains(t, text, "4")
	})

	t.Run("system as content blocks", func(t *testing.T) {
		req := &AnthropicMessagesRequest{
			System: json.RawMessage(`[{"type":"text","text":"System prompt"}]`),
			Messages: []AnthropicMessage{
				{Role: "user", Content: json.RawMessage(`"hello"`)},
			},
		}
		text := req.PromptText()
		assert.Contains(t, text, "System prompt")
		assert.Contains(t, text, "hello")
	})

t.Run("no system", func(t *testing.T) {
			req := &AnthropicMessagesRequest{
				Messages: []AnthropicMessage{
					{Role: "user", Content: json.RawMessage(`"hello"`)},
				},
			}
			text := req.PromptText()
			assert.Equal(t, "hello", text)
		})
	}

	func TestAnthropicToolUnmarshalJSON(t *testing.T) {
		t.Run("native Anthropic format", func(t *testing.T) {
			var tool AnthropicTool
			err := json.Unmarshal([]byte(`{
				"name": "get_weather",
				"description": "Get the weather",
				"input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}
			}`), &tool)
			require.NoError(t, err)
			assert.Equal(t, "get_weather", tool.Name)
			assert.Equal(t, "Get the weather", tool.Description)
			assert.JSONEq(t, `{"type":"object","properties":{"city":{"type":"string"}}}`, string(tool.InputSchema))
		})

		t.Run("OpenAI Chat format", func(t *testing.T) {
			var tool AnthropicTool
			err := json.Unmarshal([]byte(`{
				"type": "function",
				"function": {
					"name": "write_file",
					"description": "write",
					"parameters": {"type": "object", "properties": {"path": {"type": "string"}}, "required": ["path"]}
				}
			}`), &tool)
			require.NoError(t, err)
			assert.Equal(t, "write_file", tool.Name)
			assert.Equal(t, "write", tool.Description)
			assert.JSONEq(t, `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`, string(tool.InputSchema))
		})

		t.Run("empty name both formats", func(t *testing.T) {
			var tool AnthropicTool
			err := json.Unmarshal([]byte(`{"type": "unknown"}`), &tool)
			require.NoError(t, err)
			assert.Empty(t, tool.Name)
		})
	}
