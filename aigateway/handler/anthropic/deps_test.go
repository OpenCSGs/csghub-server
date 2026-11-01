package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestAnthropicRequestToLLMLogRequestKeepsAssistantToolCalls(t *testing.T) {
	req := &types.AnthropicMessagesRequest{
		Model:     "claude-3",
		MaxTokens: 1024,
		Tools: []types.AnthropicTool{
			{
				Name:        "Bash",
				Description: "Run a shell command",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
			},
		},
		Messages: []types.AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"List files"`)},
			{
				Role:    "assistant",
				Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]`),
			},
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_1","content":"main.go"}]`),
			},
		},
	}

	logReq, err := anthropicRequestToLLMLogRequest(req)
	require.NoError(t, err)
	require.Len(t, logReq.Messages, 3)

	assistant := logReq.Messages[1]
	require.NotNil(t, assistant.OfAssistant)
	require.Len(t, assistant.OfAssistant.ToolCalls, 1,
		"assistant tool_calls must survive the LLM log conversion")
	toolCall := assistant.OfAssistant.ToolCalls[0]
	require.NotNil(t, toolCall.OfFunction)
	assert.Equal(t, "toolu_1", toolCall.OfFunction.ID)
	assert.Equal(t, "function", string(toolCall.OfFunction.Type))
	assert.Equal(t, "Bash", toolCall.OfFunction.Function.Name)
	assert.Equal(t, `{"command":"ls"}`, toolCall.OfFunction.Function.Arguments)

	toolMsg := logReq.Messages[2]
	require.NotNil(t, toolMsg.OfTool)
	assert.Equal(t, "toolu_1", toolMsg.OfTool.ToolCallID)
}
