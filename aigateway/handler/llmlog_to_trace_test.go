package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestLLMLogMessagesToGenerationMessages_Nil(t *testing.T) {
	require.Nil(t, llmlogMessagesToGenerationMessages(nil))
}

func TestLLMLogMessagesToGenerationMessages_Empty(t *testing.T) {
	require.Nil(t, llmlogMessagesToGenerationMessages([]commontypes.LLMLogMessage{}))
}

func TestLLMLogMessagesToGenerationMessages_UserMessage(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "user", Content: "hello"},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "user", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "text", result[0].Parts[0].Kind)
	require.Equal(t, "hello", result[0].Parts[0].Text)
}

func TestLLMLogMessagesToGenerationMessages_AssistantWithReasoning(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "assistant", Content: "answer", ReasoningContent: "thinking process"},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "assistant", result[0].Role)
	require.Len(t, result[0].Parts, 2)
	require.Equal(t, "thinking", result[0].Parts[0].Kind)
	require.Equal(t, "thinking process", result[0].Parts[0].Thinking)
	require.Equal(t, "text", result[0].Parts[1].Kind)
	require.Equal(t, "answer", result[0].Parts[1].Text)
}

func TestLLMLogMessagesToGenerationMessages_ToolCall(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "tool_call", Content: `{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}`},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "assistant", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "tool_call", result[0].Parts[0].Kind)
	require.Equal(t, "get_weather", result[0].Parts[0].ToolCall.Name)
	require.Equal(t, `{"city":"SF"}`, string(result[0].Parts[0].ToolCall.InputJSON))
}

func TestLLMLogMessagesToGenerationMessages_ToolCallObjectArguments(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "tool_call", Content: `{"name":"get_weather","arguments":{"city":"SF"}}`},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "assistant", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "tool_call", result[0].Parts[0].Kind)
	require.Equal(t, "get_weather", result[0].Parts[0].ToolCall.Name)
	require.Equal(t, `{"city":"SF"}`, string(result[0].Parts[0].ToolCall.InputJSON))
}

func TestLLMLogMessagesToGenerationMessages_ToolCallPlainStringArguments(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "tool_call", Content: `{"name":"search","arguments":"query"}`},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "search", result[0].Parts[0].ToolCall.Name)
	require.Equal(t, `"query"`, string(result[0].Parts[0].ToolCall.InputJSON))
}

func TestLLMLogMessagesToGenerationMessages_ToolCallInvalidJSON(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "tool_call", Content: `not-json`},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "assistant", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "text", result[0].Parts[0].Kind)
	require.Equal(t, "not-json", result[0].Parts[0].Text)
}

func TestLLMLogMessagesToGenerationMessages_ToolResponse(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "tool_response", Content: `{"result": "ok"}`},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "tool", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "text", result[0].Parts[0].Kind)
	require.Equal(t, `{"result": "ok"}`, result[0].Parts[0].Text)
}

func TestLLMLogMessagesToGenerationMessages_MessageWithoutContent(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "system", Content: ""},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 0)
}

func TestLLMLogMessagesToGenerationMessages_PreservesSystemRoleForSpanAttributes(t *testing.T) {
	msgs := []commontypes.LLMLogMessage{
		{Role: "system", Content: "Follow repository instructions."},
	}
	result := llmlogMessagesToGenerationMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, "system", result[0].Role)
	require.Len(t, result[0].Parts, 1)
	require.Equal(t, "text", result[0].Parts[0].Kind)
	require.Equal(t, "Follow repository instructions.", result[0].Parts[0].Text)
}
