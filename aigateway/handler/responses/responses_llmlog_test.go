package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestResponsesLLMLogRecorderRecordNormalizesInputAndOutput(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:        "public-model",
		Instructions: json.RawMessage(`"be brief"`),
		Input: json.RawMessage(`[
			{"role":"user","content":[{"type":"input_text","text":"weather?"}]},
			{"type":"function_call","name":"lookup","arguments":"{\"city\":\"sf\"}"},
			{"type":"function_call_output","output":{"temp":70}}
		]`),
		Tools: json.RawMessage(`[{"type":"function","name":"lookup"}]`),
	}
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", req, map[string]any{"api": "/v1/responses"})
	require.NoError(t, err)
	recorder.CaptureResponse(&types.ResponsesResponse{
		ID:         "resp_public",
		Object:     "response",
		Status:     "completed",
		OutputText: "sunny",
		Output: []types.ResponsesOutputItem{{
			Type:      "function_call",
			Name:      "lookup",
			Arguments: `{"city":"sf"}`,
		}, {
			Type: "message",
			Role: "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: "sunny",
			}},
		}},
	})

	record, err := recorder.Record(&token.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6})

	require.NoError(t, err)
	require.Equal(t, "responses", record.SampleType)
	require.Equal(t, "backend-model", record.ModelID)
	require.Equal(t, "user-1", record.UserUUID)
	require.JSONEq(t, `[{"type":"function","name":"lookup"}]`, string(record.Tools))
	require.Equal(t, commontypes.LLMLogUsage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}, record.Usage)
	require.Equal(t, "/v1/responses", record.Metadata["api"])
	require.Equal(t, []string{"system", "user", "tool_call", "tool_response", "tool_call", "assistant"}, llmLogRoles(record.Messages))
	require.Equal(t, "be brief", record.Messages[0].Content)
	require.Equal(t, "weather?", record.Messages[1].Content)
	require.JSONEq(t, `{"name":"lookup","arguments":"{\"city\":\"sf\"}"}`, record.Messages[2].Content)
	require.JSONEq(t, `{"temp":70}`, record.Messages[3].Content)
	require.JSONEq(t, `{"name":"lookup","arguments":"{\"city\":\"sf\"}"}`, record.Messages[4].Content)
	require.Equal(t, "sunny", record.Messages[5].Content)
	traceInfo := recorder.TraceInfo()
	require.Equal(t, "resp_public", traceInfo.ResponseID)
	require.Empty(t, traceInfo.FinishReasons)
}

func TestResponsesLLMLogRecorderCapturesStreamPayloads(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`"hello"`),
	}, nil)
	require.NoError(t, err)

	recorder.CapturePayload(map[string]any{
		"type":        "response.output_text.delta",
		"response_id": "resp_1",
		"delta":       "hello",
	})
	recorder.CapturePayload(map[string]any{
		"type":        "response.refusal.delta",
		"response_id": "resp_1",
		"delta":       " blocked",
	})
	recorder.CapturePayload(map[string]any{
		"type":        "response.output_item.added",
		"response_id": "resp_1",
		"item": map[string]any{
			"id":        "call_1",
			"type":      "function_call",
			"name":      "lookup",
			"arguments": "",
		},
	})
	recorder.CapturePayload(map[string]any{
		"type":        "response.function_call_arguments.delta",
		"response_id": "resp_1",
		"item_id":     "call_1",
		"delta":       `{"q":"sf"}`,
	})

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"user", "tool_call", "assistant"}, llmLogRoles(record.Messages))
	require.Equal(t, "hello", record.Messages[0].Content)
	require.JSONEq(t, `{"name":"lookup","arguments":"{\"q\":\"sf\"}"}`, record.Messages[1].Content)
	require.Equal(t, "hello\nblocked", record.Messages[2].Content)
	require.Equal(t, "resp_1", recorder.TraceInfo().ResponseID)
}

func TestResponsesLLMLogRecorderCapturesCompletedToolCallAfterPartialDelta(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`"hello"`),
	}, nil)
	require.NoError(t, err)

	recorder.CapturePayload(map[string]any{
		"type":        "response.output_item.added",
		"response_id": "resp_1",
		"item": map[string]any{
			"id":   "call_1",
			"type": "function_call",
			"name": "lookup",
		},
	})
	recorder.CapturePayload(map[string]any{
		"type":        "response.function_call_arguments.delta",
		"response_id": "resp_1",
		"item_id":     "call_1",
		"delta":       `{"q":`,
	})
	recorder.CapturePayload(map[string]any{
		"type":        "response.function_call_arguments.done",
		"response_id": "resp_1",
		"item_id":     "call_1",
		"arguments":   `{"q":"sf"}`,
	})

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"user", "tool_call"}, llmLogRoles(record.Messages))
	require.JSONEq(t, `{"name":"lookup","arguments":"{\"q\":\"sf\"}"}`, record.Messages[1].Content)
}

func TestResponsesLLMLogRecorderUsesCallIDFallbackForToolCalls(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`"hello"`),
	}, nil)
	require.NoError(t, err)

	recorder.CapturePayload(map[string]any{
		"type":        "response.output_item.done",
		"response_id": "resp_1",
		"item": map[string]any{
			"call_id":   "call_1",
			"type":      "function_call",
			"name":      "lookup",
			"arguments": `{"q":"sf"}`,
		},
	})

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"user", "tool_call"}, llmLogRoles(record.Messages))
	require.JSONEq(t, `{"name":"lookup","arguments":"{\"q\":\"sf\"}"}`, record.Messages[1].Content)
}

func TestResponsesLLMLogRecorderNormalizesFallbackInputsAndOutputs(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`{"prompt":"hello"}`),
	}, nil)
	require.NoError(t, err)
	recorder.CaptureResponse(&types.ResponsesResponse{
		ID:         "resp_public",
		Object:     "response",
		Status:     "completed",
		OutputText: "fallback output",
	})

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"user", "assistant"}, llmLogRoles(record.Messages))
	require.JSONEq(t, `{"prompt":"hello"}`, record.Messages[0].Content)
	require.Equal(t, "fallback output", record.Messages[1].Content)
}

func TestResponsesLLMLogRecorderNormalizesRefusalOnlyOutput(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`"hello"`),
	}, nil)
	require.NoError(t, err)
	recorder.CaptureResponse(&types.ResponsesResponse{
		ID:     "resp_public",
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type: "message",
			Role: "assistant",
			Content: []types.ResponsesContentPart{{
				Type:    "refusal",
				Refusal: "blocked",
			}},
		}},
	})

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"user", "assistant"}, llmLogRoles(record.Messages))
	require.Equal(t, "blocked", record.Messages[1].Content)
}

func TestResponsesLLMLogRecorderPreservesInvalidFunctionCallArguments(t *testing.T) {
	recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", &types.ResponsesRequest{
		Model: "public-model",
		Input: json.RawMessage(`[
			{"type":"function_call","name":"lookup","arguments":"not-json"}
		]`),
	}, nil)
	require.NoError(t, err)

	record, err := recorder.Record(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"tool_call"}, llmLogRoles(record.Messages))
	require.JSONEq(t, `{"name":"lookup","arguments":"not-json"}`, record.Messages[0].Content)
}

func TestResponsesLLMLogRecorderTraceInfoStatuses(t *testing.T) {
	for _, status := range []string{"completed", "failed", "incomplete"} {
		t.Run(status, func(t *testing.T) {
			recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", nil, nil)
			require.NoError(t, err)

			recorder.CaptureResponse(&types.ResponsesResponse{
				ID:     "resp_public",
				Object: "response",
				Status: status,
			})

			traceInfo := recorder.TraceInfo()
			require.Equal(t, "resp_public", traceInfo.ResponseID)
			require.Empty(t, traceInfo.FinishReasons)
		})
	}
}

func TestResponsesLLMLogRecorderDerivesFinishReasonFromTerminalStatus(t *testing.T) {
	newRecorder := func() *LLMLogRecorder {
		recorder, err := NewLLMLogRecorder("req-1", "backend-model", "user-1", nil, nil)
		require.NoError(t, err)
		return recorder
	}

	t.Run("intermediate status is ignored", func(t *testing.T) {
		recorder := newRecorder()
		// Simulates a response.created event mid-stream.
		recorder.CapturePayloadMap(map[string]any{
			"type": "response.created",
			"response": map[string]any{
				"id":     "resp_1",
				"object": "response",
				"status": "in_progress",
			},
		})
		require.Empty(t, recorder.TraceInfo().FinishReasons)
	})

	t.Run("completed sets stop", func(t *testing.T) {
		recorder := newRecorder()
		recorder.CapturePayloadMap(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_1",
				"object": "response",
				"status": "completed",
			},
		})
		require.Equal(t, []string{"stop"}, recorder.TraceInfo().FinishReasons)
	})

	t.Run("completed with tool calls", func(t *testing.T) {
		recorder := newRecorder()
		// Simulate the stream event sequence: output_item.added first,
		// then response.completed. The flag is set during item processing.
		recorder.CapturePayloadMap(map[string]any{
			"type": "response.output_item.added",
			"item": map[string]any{
				"id":        "call_1",
				"type":      "function_call",
				"name":      "exec_command",
				"arguments": `{"cmd":"ls"}`,
			},
		})
		recorder.CapturePayloadMap(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_1",
				"object": "response",
				"status": "completed",
			},
		})
		require.Equal(t, []string{"tool_calls"}, recorder.TraceInfo().FinishReasons)
	})

	t.Run("incomplete with reason", func(t *testing.T) {
		recorder := newRecorder()
		recorder.CapturePayloadMap(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_1",
				"object": "response",
				"status": "incomplete",
				"incomplete_details": map[string]any{
					"reason": "max_output_tokens",
				},
			},
		})
		require.Equal(t, []string{"max_output_tokens"}, recorder.TraceInfo().FinishReasons)
	})

	t.Run("non-stream native response derives stop", func(t *testing.T) {
		recorder := newRecorder()
		recorder.CapturePayloadMap(map[string]any{
			"id":     "resp_1",
			"object": "response",
			"status": "completed",
		})
		require.Equal(t, []string{"stop"}, recorder.TraceInfo().FinishReasons)
	})
}

func llmLogRoles(messages []commontypes.LLMLogMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, msg := range messages {
		roles = append(roles, msg.Role)
	}
	return roles
}
