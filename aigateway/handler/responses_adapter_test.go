package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestValidateResponsesAdapterRequestStoreFalseSucceeds(t *testing.T) {
	store := false
	req := &types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`), Store: &store}
	require.NoError(t, validateResponsesAdapterRequest(req))
}

func TestValidateResponsesAdapterRequestRejectsStatefulFeatures(t *testing.T) {
	store := true
	req := &types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`), Store: &store}
	err := validateResponsesAdapterRequest(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported_feature:store")

	req = &types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`), PreviousResponseID: "resp_agw_v1.k1.x"}
	err = validateResponsesAdapterRequest(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported_feature:previous_response_id")
}

func TestValidateResponsesAdapterRequestAllowsReasoning(t *testing.T) {
	req := &types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`), Reasoning: json.RawMessage(`{"effort":"high"}`)}
	require.NoError(t, validateResponsesAdapterRequest(req))
}

func TestApplyAdapterReasoningRequest(t *testing.T) {
	ctx := context.Background()
	deepSeekMetadata := &commontypes.UpstreamMetadata{
		ResponsesChatAdapter: &commontypes.ResponsesChatAdapter{
			ReasoningRequest: &commontypes.ReasoningRequestConfig{
				Enabled:      true,
				EffortField:  "",
				EnableExtra:  map[string]any{"thinking": map[string]any{"type": "enabled"}},
				DisableExtra: map[string]any{"thinking": map[string]any{"type": "disabled"}},
			},
		},
	}
	glmMetadata := &commontypes.UpstreamMetadata{
		ResponsesChatAdapter: &commontypes.ResponsesChatAdapter{
			ReasoningRequest: &commontypes.ReasoningRequestConfig{
				Enabled:      true,
				EffortField:  "reasoning_effort",
				EnableExtra:  map[string]any{"enable_thinking": true},
				DisableExtra: map[string]any{"enable_thinking": false},
			},
		},
	}
	disabledMetadata := &commontypes.UpstreamMetadata{
		ResponsesChatAdapter: &commontypes.ResponsesChatAdapter{
			ReasoningRequest: &commontypes.ReasoningRequestConfig{
				Enabled: false,
			},
		},
	}
	genericMetadata := &commontypes.UpstreamMetadata{
		ResponsesChatAdapter: &commontypes.ResponsesChatAdapter{
			ReasoningRequest: &commontypes.ReasoningRequestConfig{
				Enabled:     true,
				EffortField: "reasoning_effort",
			},
		},
	}

	t.Run("config absent", func(t *testing.T) {
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, nil, json.RawMessage(`{"effort":"high"}`))
		require.NoError(t, err)
		require.Empty(t, chatReq.RawJSON)
	})

	t.Run("enabled false effort none", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(disabledMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"none"}`))
		require.NoError(t, err)
		require.Empty(t, chatReq.RawJSON)
	})

	t.Run("enabled false effort high", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(disabledMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"high"}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported_feature:reasoning")
	})

	t.Run("enabled true effort none merges disable extra", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(deepSeekMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"none"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.JSONEq(t, `{"type":"disabled"}`, string(raw["thinking"]))
		_, hasEffort := raw["reasoning_effort"]
		require.False(t, hasEffort)
	})

	t.Run("enabled true effort low", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(glmMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"low"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"low"`, string(raw["reasoning_effort"]))
		require.Equal(t, `true`, string(raw["enable_thinking"]))
	})

	t.Run("enabled true effort xhigh maps to max", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(glmMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"xhigh"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"max"`, string(raw["reasoning_effort"]))
	})

	t.Run("enabled true effort minimal", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"minimal"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"minimal"`, string(raw["reasoning_effort"]))
	})

	t.Run("enabled true unknown effort", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"foobar"}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), `invalid reasoning effort: "foobar"`)
	})

	t.Run("effort normalized case and whitespace", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":" High "}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"high"`, string(raw["reasoning_effort"]))
	})

	t.Run("reasoning object without effort", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(glmMetadata)
		chatReq := &types.ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"summary":"auto"}`))
		require.NoError(t, err)
		require.Empty(t, chatReq.RawJSON)
	})

	t.Run("responsesToChatRequest forwards reasoning with metadata", func(t *testing.T) {
		req := &types.ResponsesRequest{
			Model:     "m",
			Input:     json.RawMessage(`"hi"`),
			Reasoning: json.RawMessage(`{"effort":"none"}`),
		}
		chatReq, err := responsesToChatRequest(ctx, req, "upstream-model", deepSeekMetadata)
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.JSONEq(t, `{"type":"disabled"}`, string(raw["thinking"]))
	})
}

func TestNormalizeChatRole(t *testing.T) {
	cases := map[string]string{
		"":          "user",
		"developer": "system",
		"system":    "system",
		"user":      "user",
		"assistant": "assistant",
		"tool":      "tool",
	}
	for role, want := range cases {
		require.Equal(t, want, normalizeChatRole(role))
	}
}

func TestResponsesToChatRequestMapsStringInputAndInstructions(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:        "public",
		Instructions: json.RawMessage(`"be concise"`),
		Input:        json.RawMessage(`"hello"`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)
	require.Equal(t, "upstream-model", chatReq.Model)
	require.Len(t, chatReq.Messages, 2)
}

func TestResponsesToChatRequestIgnoresNonStringInstructions(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:        "public",
		Instructions: json.RawMessage(`{"text":"do not stringify me"}`),
		Input:        json.RawMessage(`"hello"`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{"role": "user", "content": "hello"}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMapsInputImageStringURL(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{
				"type": "message",
				"role": "user",
				"content": [
					{"type": "input_text", "text": "look"},
					{"type": "input_image", "image_url": "https://example.test/a.png"}
				]
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "look"},
				{"type": "image_url", "image_url": {"url": "https://example.test/a.png"}}
			]
		}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMapsFunctionCallOutputInputImage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{
				"type": "function_call",
				"call_id": "call_view_image_1",
				"name": "view_image",
				"arguments": "{\"path\":\"/tmp/image.png\"}"
			},
			{
				"type": "function_call_output",
				"call_id": "call_view_image_1",
				"output": [
					{"type": "input_image", "image_url": "data:image/jpeg;base64,abc", "detail": "high"}
				]
			},
			{
				"role": "user",
				"content": [
					{"type": "input_text", "text": "describe it"}
				]
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [{
					"id": "call_view_image_1",
					"type": "function",
					"function": {"name": "view_image", "arguments": "{\"path\":\"/tmp/image.png\"}"}
				}]
			},
			{
				"role": "tool",
				"tool_call_id": "call_view_image_1",
				"content": [
					{"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,abc", "detail": "high"}}
				]
			},
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "describe it"}
				]
			}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestAttachesReasoningInputToAssistantMessage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]},
			{"type":"message","role":"assistant","content":"answer"},
			{"type":"message","role":"user","content":"continue"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{"role": "assistant", "content": "answer", "reasoning_content": "thinking"},
			{"role": "user", "content": "continue"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestAttachesReasoningInputToFunctionCall(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"reasoning","content":[{"type":"reasoning_text","text":"plan tool"}]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{
				"role": "assistant",
				"content": "",
				"reasoning_content": "plan tool",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}
				}]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "ok"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMergesParallelFunctionCallsIntoOneAssistantMessage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":"run both"},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call","call_id":"call_2","name":"fetch","arguments":"{\"u\":\"y\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"r1"},
			{"type":"function_call_output","call_id":"call_2","output":"r2"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{"role": "user", "content": "run both"},
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{"id": "call_1", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}},
					{"id": "call_2", "type": "function", "function": {"name": "fetch", "arguments": "{\"u\":\"y\"}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "r1"},
			{"role": "tool", "tool_call_id": "call_2", "content": "r2"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestDoesNotMergeFunctionCallsAcrossToolReplies(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"r1"},
			{"type":"function_call","call_id":"call_2","name":"fetch","arguments":"{\"u\":\"y\"}"},
			{"type":"function_call_output","call_id":"call_2","output":"r2"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{"id": "call_1", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "r1"},
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{"id": "call_2", "type": "function", "function": {"name": "fetch", "arguments": "{\"u\":\"y\"}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_2", "content": "r2"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestAttachesReasoningRecordedAfterFunctionCall(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":"go"},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"why lookup"}]},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"function_call","call_id":"call_2","name":"save","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_2","output":"saved"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{"role": "user", "content": "go"},
			{
				"role": "assistant",
				"content": "",
				"reasoning_content": "why lookup",
				"tool_calls": [
					{"id": "call_1", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "ok"},
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{"id": "call_2", "type": "function", "function": {"name": "save", "arguments": "{}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_2", "content": "saved"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMergesParallelCallsWithReasoningBeforeOutputs(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call","call_id":"call_2","name":"fetch","arguments":"{\"u\":\"y\"}"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"planned both"}]},
			{"type":"function_call_output","call_id":"call_1","output":"r1"},
			{"type":"function_call_output","call_id":"call_2","output":"r2"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{
				"role": "assistant",
				"content": "",
				"reasoning_content": "planned both",
				"tool_calls": [
					{"id": "call_1", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}},
					{"id": "call_2", "type": "function", "function": {"name": "fetch", "arguments": "{\"u\":\"y\"}"}}
				]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "r1"},
			{"role": "tool", "tool_call_id": "call_2", "content": "r2"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestDropsOrphanReasoningWithNoAssistantTarget(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMergesTrailingReasoningIntoAssistantMessage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"message","role":"assistant","content":"answer"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"afterthought"}]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{"role": "assistant", "content": "answer", "reasoning_content": "afterthought"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMergesTrailingReasoningIntoMostRecentAssistantThroughTool(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"plan"}]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"afterthought"}]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [
			{
				"role": "assistant",
				"content": "",
				"reasoning_content": "plan\nafterthought",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "lookup", "arguments": "{\"q\":\"x\"}"}
				}]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "ok"}
		],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestDropsTrailingReasoningAfterUserMessage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":"hi"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"orphan"}]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{"role": "user", "content": "hi"}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestIgnoresEncryptedReasoningInput(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{"type":"reasoning","encrypted_content":"opaque"},
			{"type":"message","role":"user","content":"hello"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{"role": "user", "content": "hello"}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestDefaultsParallelToolCalls(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
	require.JSONEq(t, `true`, string(raw["parallel_tool_calls"]))
}

func TestResponsesAdapterChatRequestIncludesStreamUsage(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:  "public",
		Input:  json.RawMessage(`"hello"`),
		Stream: true,
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)
	chatReq.StreamOptions = &types.StreamOptions{IncludeUsage: true}

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{"role": "user", "content": "hello"}],
		"stream": true,
		"stream_options": {"include_usage": true},
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestPreservesParallelToolCallsFalse(t *testing.T) {
	parallel := false
	req := &types.ResponsesRequest{
		Model:             "public",
		Input:             json.RawMessage(`"hello"`),
		ParallelToolCalls: &parallel,
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
	require.JSONEq(t, `false`, string(raw["parallel_tool_calls"]))
}

func TestResponsesToChatRequestMapsFunctionTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"What is the weather in Tokyo?"`),
		Tools: json.RawMessage(`[
			{
				"type": "function",
				"name": "get_weather",
				"description": "Get current weather",
				"parameters": {
					"type": "object",
					"properties": {
						"city": {"type": "string"}
					},
					"required": ["city"]
				}
			}
		]`),
	}

	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get current weather",
				"parameters": {
					"type": "object",
					"properties": {
						"city": {"type": "string"}
					},
					"required": ["city"]
				}
			}
		}
	]`, string(data))
}

func TestResponsesToChatRequestMapsFunctionToolWithoutType(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"What is the weather in Tokyo?"`),
		Tools: json.RawMessage(`[
			{
				"name": "get_weather",
				"parameters": {"type": "object"}
			}
		]`),
	}

	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "get_weather",
				"parameters": {"type": "object"}
			}
		}
	]`, string(data))
}

func TestResponsesToChatRequestNormalizesNestedFunctionToolWithoutType(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"What is the weather in Tokyo?"`),
		Tools: json.RawMessage(`[
			{
				"function": {
					"name": "get_weather",
					"parameters": {"type": "object"}
				}
			}
		]`),
	}

	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "get_weather",
				"parameters": {"type": "object"}
			}
		}
	]`, string(data))
}

func TestResponsesToChatRequestRejectsInvalidNestedFunctionTool(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"What is the weather in Tokyo?"`),
		Tools: json.RawMessage(`[
			{
				"type": "function",
				"function": null
			}
		]`),
	}

	_, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "function must be an object")
}

func TestResponsesToChatRequestMapsInputAudioContentPart(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{
				"type": "message",
				"role": "user",
				"content": [
					{"type": "input_audio", "input_audio": {"data": "abc", "format": "wav"}}
				]
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{
			"role": "user",
			"content": [{
				"type": "input_audio",
				"input_audio": {"data": "abc", "format": "wav"}
			}]
		}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestResponsesToChatRequestMapsFlatInputAudioContentPart(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`[
			{
				"type": "message",
				"role": "user",
				"content": [
					{"type": "input_audio", "audio": "abc", "format": "mp3"}
				]
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "upstream-model",
		"messages": [{
			"role": "user",
			"content": [{
				"type": "input_audio",
				"input_audio": {"data": "abc", "format": "mp3"}
			}]
		}],
		"parallel_tool_calls": true
	}`, string(body))
}

func TestChatResponseToResponsesMapsUsageAndText(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"hello"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Equal(t, "public-model", resp.Model)
	require.Equal(t, "hello", resp.OutputText)
	require.Equal(t, int64(2), resp.Usage.InputTokens)
	require.Equal(t, int64(3), resp.Usage.OutputTokens)
	require.Nil(t, resp.ParallelToolCalls)
	require.Len(t, resp.Output, 1)
	require.Empty(t, resp.Output[0].Summary)
}

func TestChatResponseToResponsesMapsReasoningContent(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"thinking"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Equal(t, "answer", resp.OutputText)
	require.Len(t, resp.Output, 2)
	require.Equal(t, "message", resp.Output[0].Type)
	require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
	require.Equal(t, "answer", resp.Output[0].Content[0].Text)
	require.Equal(t, "reasoning", resp.Output[1].Type)
	require.Len(t, resp.Output[1].Summary, 1)
	require.Equal(t, "summary_text", resp.Output[1].Summary[0].Type)
	require.Equal(t, "thinking", resp.Output[1].Summary[0].Text)
}

func TestChatResponseToResponsesMapsReasoningFallbackField(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"answer","reasoning":"chain"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Len(t, resp.Output, 2)
	require.Equal(t, "reasoning", resp.Output[1].Type)
	require.Equal(t, "chain", resp.Output[1].Summary[0].Text)
}

func TestChatResponseToResponsesPrefersReasoningContent(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"preferred","reasoning":"fallback"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Len(t, resp.Output, 2)
	require.Equal(t, "preferred", resp.Output[1].Summary[0].Text)
}

func TestChatResponseToResponsesTrimsReasoningContent(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"  thinking\n"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Len(t, resp.Output, 2)
	require.Equal(t, "thinking", resp.Output[1].Summary[0].Text)
}

func TestDecodeResponsesAdapterChatBodyDecodesGzip(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"hello"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`)
	encoded, err := compress.Encode("gzip", body)
	require.NoError(t, err)

	writer := newBufferCommonResponseWriter()
	writer.Header().Set("Content-Encoding", "gzip")
	_, err = writer.Write(encoded)
	require.NoError(t, err)

	decoded, err := decodeResponsesAdapterChatBody(writer)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(decoded))
}

func TestResponsesAdapterNonStreamWriterFinalizesResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
	writer := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", counter, nil, "")
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","content":"hello"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`))
	require.NoError(t, err)

	require.NoError(t, writer.Finalize(http.StatusOK))
	require.Equal(t, http.StatusOK, w.Code)
	var resp types.ResponsesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "public-model", resp.Model)
	require.Equal(t, "hello", resp.OutputText)
	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
}

func extractResponsesOutputItem(t *testing.T, data []byte, index int) []byte {
	t.Helper()
	var body struct {
		Output []json.RawMessage `json:"output"`
	}
	require.NoError(t, json.Unmarshal(data, &body))
	require.Greater(t, len(body.Output), index)
	return body.Output[index]
}

// responsesToolAliasesForNamespacesTest builds a resolver for unambiguous
// namespaced tools: each tool's upstream name equals its original function name.
func responsesToolAliasesForNamespacesTest(t *testing.T, namespaces map[string]string) *responsesToolAliases {
	t.Helper()
	aliases := newResponsesToolAliases()
	for functionName, namespaceName := range namespaces {
		require.NoError(t, aliases.register(responsesToolIdentity{Name: functionName, Namespace: namespaceName}, functionName))
	}
	return aliases
}

func TestResponsesAdapterNonStreamWriterRestoresNamespaceToolCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.toolAliases = responsesToolAliasesForNamespacesTest(t, map[string]string{"get_top_download_models": "mcp__csghub_production"})
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{
			"message":{
				"role":"assistant",
				"tool_calls":[{
					"id":"call_1",
					"type":"function",
					"function":{"name":"get_top_download_models","arguments":"{\"num\":20}"}
				}]
			},
			"finish_reason":"tool_calls"
		}]
	}`))
	require.NoError(t, err)

	require.NoError(t, writer.Finalize(http.StatusOK))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{
		"id": "call_1",
		"type": "function_call",
		"status": "completed",
		"call_id": "call_1",
		"name": "get_top_download_models",
		"namespace": "mcp__csghub_production",
		"arguments": "{\"num\":20}"
	}`, string(extractResponsesOutputItem(t, w.Body.Bytes(), 0)))
}

func TestResponsesAdapterNonStreamWriterForwardsUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusBadRequest)
	errorBody := []byte(`{"error":{"message":"invalid request"}}`)
	_, err := writer.Write(errorBody)
	require.NoError(t, err)

	require.NoError(t, writer.Finalize(http.StatusBadRequest))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, string(errorBody), w.Body.String())
}

func TestChatResponseToResponsesMapsRefusal(t *testing.T) {
	resp, err := chatResponseToResponses([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{"message":{"role":"assistant","refusal":"I can't help with that."}}],
		"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
	}`), "public-model")
	require.NoError(t, err)
	require.Len(t, resp.Output, 1)
	require.Len(t, resp.Output[0].Content, 1)
	require.Equal(t, "refusal", resp.Output[0].Content[0].Type)
	require.Equal(t, "I can't help with that.", resp.Output[0].Content[0].Refusal)
}

func TestResponsesCounterFallsBackToUsageDetails(t *testing.T) {
	usage := &types.ResponsesUsage{
		InputTokensDetails: &types.ResponsesInputTokenDetails{
			CachedTokens:         12,
			CachedCreationTokens: 3,
			TextTokens:           5,
		},
		OutputTokensDetails: &types.ResponsesOutputTokenDetails{
			TextTokens:      7,
			ReasoningTokens: 11,
		},
	}

	counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
	counter.Response(&types.ResponsesResponse{Usage: usage})
	tokenUsage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(20), tokenUsage.PromptTokens)
	require.Equal(t, int64(18), tokenUsage.CompletionTokens)
	require.Equal(t, int64(38), tokenUsage.TotalTokens)
	require.Equal(t, int64(12), tokenUsage.CachedPromptTokens)
	require.Equal(t, int64(3), tokenUsage.CacheCreationPromptTokens)
	require.Equal(t, int64(11), tokenUsage.ReasoningTokens)
}

func TestResponsesAdapterStreamWriterEmitsResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"he"},"index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"llo"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, `"type":"response.created"`)
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, `"type":"response.output_text.delta"`)
	require.Contains(t, body, `"item_id":"msg_0"`)
	require.Contains(t, body, `"delta":"he"`)
	require.Contains(t, body, `"text":"hello"`)
	require.Contains(t, body, "event: response.content_part.done")
	require.Contains(t, body, `"output_text":"hello"`)
	require.Contains(t, body, "event: response.completed")
	require.Contains(t, body, `"type":"response.completed"`)
	require.Contains(t, body, "data: [DONE]")
}

func TestResponsesAdapterStreamWriterEmitsReasoningContentEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"reasoning_content":"think "},"index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"answer","reasoning_content":"more"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.reasoning_summary_text.delta")
	require.Contains(t, body, `"type":"response.reasoning_summary_text.delta"`)
	require.Contains(t, body, `"item_id":"rs_0"`)
	require.Contains(t, body, `"delta":"think "`)
	require.Contains(t, body, `"delta":"more"`)
	require.Contains(t, body, "event: response.reasoning_summary_text.done")
	require.Contains(t, body, `"part":{"type":"summary_text","text":"think more"}`)
	require.Contains(t, body, `"type":"reasoning"`)
	require.Contains(t, body, `"summary":[{"type":"summary_text","text":"think more"}]`)
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, `"output_text":"answer"`)
}

func TestResponsesAdapterStreamWriterEmitsReasoningFallbackEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"reasoning":"chain"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.reasoning_summary_text.delta")
	require.Contains(t, body, `"delta":"chain"`)
	require.Contains(t, body, `"summary":[{"type":"summary_text","text":"chain"}]`)
}

func TestResponsesAdapterStreamWriterDoesNotEmitEmptyReasoningItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.NotContains(t, body, "response.reasoning_summary_text")
	require.NotContains(t, body, `"type":"reasoning"`)
	require.Contains(t, body, `"output_text":"hello"`)
}

func TestResponsesAdapterStreamWriterFinishResponseStreamEmitsCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	writer.finishResponseStream()
	writer.finishResponseStream()

	body := w.Body.String()
	require.Equal(t, 1, strings.Count(body, "event: response.completed"))
	require.Equal(t, 1, strings.Count(body, "data: [DONE]"))
	require.Contains(t, body, `"output_text":"hello"`)
}

func TestResponsesAdapterStreamWriterCapturesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":5,"total_tokens":9}}` + "\n\n"))
	require.NoError(t, err)
	require.NotNil(t, writer.usage)
	require.Equal(t, int64(4), writer.usage.InputTokens)
	require.Equal(t, int64(5), writer.usage.OutputTokens)
	require.Equal(t, int64(9), writer.usage.TotalTokens)
}

func TestResponsesAdapterStreamWriterAppendsResponsesEventsToCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", counter, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"content":"hello"}}]}` + "\n\n"))
	require.NoError(t, err)
	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), usage.PromptTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
}

func TestResponsesAdapterStreamWriterCompletedResponseIncludesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":48,"completion_tokens":116,"total_tokens":164}}` + "\n\n"))
	require.NoError(t, err)
	writer.finishResponseStream()

	body := w.Body.String()
	require.Contains(t, body, `"usage":{"input_tokens":48,"output_tokens":116,"total_tokens":164}`)
}

func parseResponsesAdapterSSEEvents(t *testing.T, body string) []map[string]any {
	t.Helper()
	events := []map[string]any{}
	for _, line := range strings.Split(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(data), &event))
		events = append(events, event)
	}
	return events
}

func TestResponsesAdapterStreamWriterAddsSequenceNumbers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	events := parseResponsesAdapterSSEEvents(t, w.Body.String())
	require.NotEmpty(t, events)
	for i, event := range events {
		require.Equal(t, float64(i), event["sequence_number"], "event %d (%v)", i, event["type"])
	}
}

func TestResponsesAdapterStreamWriterEmitsToolCallItemBeforeArguments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\""}}]}}]}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	added := strings.Index(body, "event: response.output_item.added")
	delta := strings.Index(body, "event: response.function_call_arguments.delta")
	require.NotEqual(t, -1, added)
	require.NotEqual(t, -1, delta)
	require.Less(t, added, delta)
	require.Contains(t, body, `"type":"function_call"`)
	require.Contains(t, body, `"call_id":"call_1"`)
	require.Contains(t, body, `"name":"lookup"`)
	require.Contains(t, body, "event: response.function_call_arguments.delta")
	require.Contains(t, body, `"item_id":"call_1"`)
}

func TestResponsesAdapterStreamWriterToolOnlyStreamDoesNotEmitTextItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Tokyo\"}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.NotContains(t, body, `"type":"message"`)
	require.NotContains(t, body, "event: response.content_part.added")
	require.NotContains(t, body, "event: response.output_text.done")
	require.Contains(t, body, `"type":"function_call"`)
	require.Contains(t, body, `"output_index":0`)
	require.Contains(t, body, "event: response.function_call_arguments.done")
	doneIdx := strings.Index(body, "event: response.function_call_arguments.done")
	require.NotEqual(t, -1, doneIdx)
	doneEnd := strings.Index(body[doneIdx:], "\n\n")
	require.NotEqual(t, -1, doneEnd)
	doneEvent := body[doneIdx : doneIdx+doneEnd]
	require.Contains(t, doneEvent, `"arguments":"{\"city\":\"Tokyo\"}"`)
	require.Contains(t, doneEvent, `"item_id":"call_1"`)
	require.Contains(t, doneEvent, `"name":"get_weather"`)
	require.Contains(t, doneEvent, `"sequence_number":`)
	require.Contains(t, body, "event: response.output_item.done")
	require.Contains(t, body, "event: response.completed")
}

func TestResponsesAdapterStreamWriterPreservesMalformedToolArguments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"broken\""}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)

	events := parseResponsesAdapterSSEEvents(t, w.Body.String())
	for _, event := range events {
		if event["type"] != "response.function_call_arguments.done" {
			continue
		}
		require.Equal(t, "lookup", event["name"])
		require.Equal(t, `{"broken"`, event["arguments"])
		return
	}
	t.Fatal("response.function_call_arguments.done event not found")
}

func TestResponsesAdapterStreamWriterRestoresNamespaceToolCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.toolAliases = responsesToolAliasesForNamespacesTest(t, map[string]string{"get_top_download_models": "mcp__csghub_production"})
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_top_download_models","arguments":"{\"num\":20}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, `"type":"function_call"`)
	require.Contains(t, body, `"name":"get_top_download_models"`)
	require.Contains(t, body, `"namespace":"mcp__csghub_production"`)
	require.Contains(t, body, `"call_id":"call_1"`)
	require.Contains(t, body, "event: response.output_item.added")
	require.Contains(t, body, "event: response.output_item.done")
	require.Contains(t, body, "event: response.completed")
}

func TestResponsesAdapterStreamWriterRestoresNamespaceWhenToolNameArrivesLate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.toolAliases = responsesToolAliasesForNamespacesTest(t, map[string]string{"get_top_download_models": "mcp__csghub_production"})
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	require.NotContains(t, w.Body.String(), "event: response.output_item.added")
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"name":"get_top_download_models"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"arguments":"{\"num\":20}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	added := strings.Index(body, "event: response.output_item.added")
	delta := strings.Index(body, "event: response.function_call_arguments.delta")
	require.NotEqual(t, -1, added)
	require.NotEqual(t, -1, delta)
	require.Less(t, added, delta)
	addedEnd := strings.Index(body[added:], "\n\n")
	require.NotEqual(t, -1, addedEnd)
	require.Contains(t, body[added:added+addedEnd], `"name":"get_top_download_models"`)
	require.Contains(t, body[added:added+addedEnd], `"namespace":"mcp__csghub_production"`)
}

func TestResponsesAdapterStreamWriterBuffersArgumentsUntilToolNameArrives(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.toolAliases = responsesToolAliasesForNamespacesTest(t, map[string]string{"get_top_download_models": "mcp__csghub_production"})
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"arguments":"{\"num\":"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	require.NotContains(t, w.Body.String(), "event: response.output_item.added")
	require.NotContains(t, w.Body.String(), "event: response.function_call_arguments.delta")

	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"name":"get_top_download_models"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"arguments":"20}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	added := strings.Index(body, "event: response.output_item.added")
	delta := strings.Index(body, "event: response.function_call_arguments.delta")
	require.NotEqual(t, -1, added)
	require.NotEqual(t, -1, delta)
	require.Less(t, added, delta)
	addedEnd := strings.Index(body[added:], "\n\n")
	require.NotEqual(t, -1, addedEnd)
	require.Contains(t, body[added:added+addedEnd], `"name":"get_top_download_models"`)
	require.Contains(t, body[added:added+addedEnd], `"namespace":"mcp__csghub_production"`)
	require.Contains(t, body, `"delta":"{\"num\":"`)
	require.Contains(t, body, `"arguments":"{\"num\":20}"`)
	require.Contains(t, body, `"name":"get_top_download_models"`)
}

func TestResponsesAdapterStreamWriterEmitsRefusalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"refusal":"blocked"},"finish_reason":"stop"}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, `"type":"refusal"`)
	require.Contains(t, body, "event: response.refusal.delta")
	require.Contains(t, body, `"item_id":"msg_0"`)
	require.Contains(t, body, `"delta":"blocked"`)
	require.Contains(t, body, "event: response.refusal.done")
	require.Contains(t, body, `"refusal":"blocked"`)
	require.Contains(t, body, "event: response.output_item.done")
	require.Contains(t, body, "event: response.completed")
	require.NotContains(t, body, "event: response.output_text.delta")
	require.NotContains(t, body, "event: response.output_text.done")
}

func TestResponsesAdapterStreamWriterRefusalTakesPrecedenceOverContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"content":"text","refusal":"blocked"},"finish_reason":"stop"}]}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.refusal.delta")
	require.NotContains(t, body, "event: response.output_text.delta")
}

func TestResponsesAdapterStreamWriterEmitsErrorAndStopsStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"partial"},"index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`event: error` + "\n" + `data: {"error":{"message":"context length exceeded","type":"invalid_request_error","code":"context_length_exceeded"}}` + "\n\n"))
	require.NoError(t, err)
	writer.finishResponseStream()

	body := w.Body.String()
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, `"delta":"partial"`)
	require.Contains(t, body, "event: error")
	require.Contains(t, body, `"message":"context length exceeded"`)
	require.Contains(t, body, `"code":"context_length_exceeded"`)
	require.NotContains(t, body, "event: response.completed")
}

func TestRecordResponsesUsageFallsBackToTokenCounter(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	model := &types.Model{BaseModel: types.BaseModel{ID: "model"}}
	modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream-model"}
	counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hey"`)})
	counter.Response(&types.ResponsesResponse{Output: []types.ResponsesOutputItem{{
		Type: "message",
		Content: []types.ResponsesContentPart{{
			Type: "output_text",
			Text: "done",
		}},
	}}})
	tokenUsage := &token.Usage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7}

	var wg sync.WaitGroup
	wg.Add(2)
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, usage *token.Usage) error {
			wg.Done()
			return nil
		}).
		Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream-model", mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.PromptTokens == tokenUsage.PromptTokens &&
				usage.CompletionTokens == tokenUsage.CompletionTokens &&
				usage.TotalTokens == tokenUsage.TotalTokens
		}), "api-key", mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, _ int64) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "api-key", nil, responsesTracePostProcessInput{StatusCode: http.StatusOK}, nil)
	wg.Wait()
}

func TestRecordResponsesUsagePrefersResponsesUsage(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	model := &types.Model{BaseModel: types.BaseModel{ID: "model"}}
	modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream-model"}
	responsesUsage := &types.ResponsesUsage{InputTokens: 5, OutputTokens: 6, TotalTokens: 11}
	counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
	counter.Response(&types.ResponsesResponse{Usage: responsesUsage})

	var wg sync.WaitGroup
	wg.Add(2)
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, usage *token.Usage) error {
			wg.Done()
			return nil
		}).
		Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream-model", mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.PromptTokens == 5 &&
				usage.CompletionTokens == 6 &&
				usage.TotalTokens == 11
		}), "api-key", mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, _ int64) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "api-key", nil, responsesTracePostProcessInput{StatusCode: http.StatusOK}, nil)
	wg.Wait()
}

func TestValidateResponsesAdapterRequestAllowsNonFunctionTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "m",
		Input: json.RawMessage(`"hi"`),
		Tools: json.RawMessage(`[
			{"type": "web_search_preview"},
			{"type": "file_search", "vector_store_ids": ["vs_123"]},
			{"type": "code_interpreter", "container": {"type": "auto"}}
		]`),
	}
	require.NoError(t, validateResponsesAdapterRequest(req))
}

func TestResponsesToChatRequestDropsNonFunctionTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type": "code_interpreter"},
			{"type": "file_search", "vector_store_ids": ["vs_1"]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotContains(t, parsed, "tools")
}

func TestResponsesToChatRequestFlattensNamespaceFunctionTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{
				"type": "namespace",
				"name": "mcp",
				"description": "CSGHub production MCP server",
				"tools": [
					{
						"type": "function",
						"name": "view_image",
						"description": "View an image",
						"parameters": {
							"type": "object",
							"properties": {"path": {"type": "string"}},
							"required": ["path"]
						}
					},
					{"type": "web_search"}
				]
			},
			{"type": "web_search"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 1)
	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "view_image",
				"description": "View an image",
				"parameters": {
					"type": "object",
					"properties": {"path": {"type": "string"}},
					"required": ["path"]
				}
			}
		}
	]`, string(data))
	_, aliases, err := responsesToChatRequestResolved(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)
	require.Equal(t, responsesToolIdentity{Name: "view_image", Namespace: "mcp"}, aliases.identity("view_image"))
}

func TestResponsesToChatRequestConvertsCallableNamespaceTool(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{
				"type": "namespace",
				"name": "exec_command",
				"description": "Run a command",
				"parameters": {
					"type": "object",
					"properties": {"cmd": {"type": "string"}},
					"required": ["cmd"]
				}
			}
		]`),
	}
	chatReq, aliases, err := responsesToChatRequestResolved(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 1)
	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "exec_command",
				"description": "Run a command",
				"parameters": {
					"type": "object",
					"properties": {"cmd": {"type": "string"}},
					"required": ["cmd"]
				}
			}
		}
	]`, string(data))
	require.Equal(t, responsesToolIdentity{Name: "exec_command", Namespace: "exec_command"}, aliases.identity("exec_command"))

	resp, err := chatResponseToResponsesWithToolAliases([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{
			"message":{
				"role":"assistant",
				"tool_calls":[{
					"id":"call_1",
					"type":"function",
					"function":{"name":"exec_command","arguments":"{\"cmd\":\"pwd\"}"}
				}]
			},
			"finish_reason":"tool_calls"
		}]
	}`), "public-model", aliases)
	require.NoError(t, err)
	require.Len(t, resp.Output, 1)
	data, err = json.Marshal(resp.Output[0])
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id": "call_1",
		"type": "function_call",
		"status": "completed",
		"call_id": "call_1",
		"name": "exec_command",
		"namespace": "exec_command",
		"arguments": "{\"cmd\":\"pwd\"}"
	}`, string(data))
}

func TestResponsesToChatRequestAcceptsDuplicateNamespaceToolNames(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{
				"type": "namespace",
				"name": "mcp_filesystem",
				"tools": [
					{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
				]
			},
			{
				"type": "namespace",
				"name": "codex_filesystem",
				"tools": [
					{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
				]
			}
		]`),
	}

	chatReq, aliases, err := responsesToChatRequestResolved(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 2)
	mcpAlias := responsesNamespacedToolAlias("mcp_filesystem", "filesystem.read")
	codexAlias := responsesNamespacedToolAlias("codex_filesystem", "filesystem.read")
	require.NotEqual(t, mcpAlias, codexAlias)

	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	var decoded []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	names := make([]string, 0, len(decoded))
	for _, tool := range decoded {
		names = append(names, tool.Function.Name)
	}
	require.ElementsMatch(t, []string{mcpAlias, codexAlias}, names)

	require.Equal(t, responsesToolIdentity{Name: "filesystem.read", Namespace: "mcp_filesystem"}, aliases.identity(mcpAlias))
	require.Equal(t, responsesToolIdentity{Name: "filesystem.read", Namespace: "codex_filesystem"}, aliases.identity(codexAlias))
	require.Equal(t, responsesToolIdentity{Name: "filesystem.read"}, aliases.identity("filesystem.read"))
}

func TestResponsesToChatRequestFlattensNestedNamespaceFunctionTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"exec_command"}}`),
		Tools: json.RawMessage(`[
			{
				"type": "namespace",
				"namespace": {
					"name": "local",
					"functions": [
						{
							"function": {
								"name": "exec_command",
								"description": "Run a command",
								"parameters": {"type": "object"}
							}
						}
					]
				}
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.JSONEq(t, `{"type":"function","function":{"name":"exec_command"}}`, string(parsed["tool_choice"]))
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "exec_command",
				"description": "Run a command",
				"parameters": {"type": "object"}
			}
		}
	]`, string(parsed["tools"]))
}

func TestResponsesToChatRequestDropsRequiredToolChoiceWhenNoFunctionToolsRemain(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`"required"`),
		Tools: json.RawMessage(`[
			{"type": "code_interpreter"},
			{"type": "file_search", "vector_store_ids": ["vs_1"]}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotContains(t, parsed, "tools")
	require.NotContains(t, parsed, "tool_choice")
}

func TestResponsesToChatRequestDropsHostedToolChoice(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"code_interpreter"}`),
		Tools: json.RawMessage(`[
			{"type": "code_interpreter"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotContains(t, parsed, "tool_choice")
}

func TestResponsesToChatRequestRejectsInvalidToolChoiceJSON(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"function"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "parameters": {"type": "object"}}
		]`),
	}
	_, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid responses tool_choice")
}

func TestResponsesToChatRequestDropsUnsupportedToolChoiceShapes(t *testing.T) {
	for _, toolChoice := range []string{`42`, `["required"]`} {
		t.Run(toolChoice, func(t *testing.T) {
			req := &types.ResponsesRequest{
				Model:      "public",
				Input:      json.RawMessage(`"hello"`),
				ToolChoice: json.RawMessage(toolChoice),
				Tools: json.RawMessage(`[
					{"type": "function", "name": "get_weather", "parameters": {"type": "object"}}
				]`),
			}
			chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
			require.NoError(t, err)

			body, err := marshalChatRequestBody(chatReq, "upstream-model")
			require.NoError(t, err)
			var parsed map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &parsed))
			require.NotContains(t, parsed, "tool_choice")
		})
	}
}

func TestResponsesToChatRequestMixedToolsDropsNonFunction(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "parameters": {"type": "object"}},
			{"type": "code_interpreter"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	// Only the function tool should remain in chatReq.Tools
	require.Len(t, chatReq.Tools, 1)
	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "get_weather",
				"parameters": {"type": "object"}
			}
		}
	]`, string(data))
}

func TestResponsesToChatRequestKeepsRequiredToolChoiceWhenFunctionToolsRemain(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`"required"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "parameters": {"type": "object"}},
			{"type": "code_interpreter"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.JSONEq(t, `"required"`, string(parsed["tool_choice"]))
}

func TestResponsesToChatRequestKeepsSurvivingFunctionToolChoice(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"get_weather"}}`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "parameters": {"type": "object"}},
			{"type": "code_interpreter"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.JSONEq(t, `{"type":"function","function":{"name":"get_weather"}}`, string(parsed["tool_choice"]))
}

func TestResponsesToChatRequestDropsMissingFunctionToolChoice(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:      "public",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"run_code"}}`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "parameters": {"type": "object"}},
			{"type": "code_interpreter"}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	body, err := marshalChatRequestBody(chatReq, "upstream-model")
	require.NoError(t, err)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotContains(t, parsed, "tool_choice")
}

func TestResponsesToChatRequestFunctionOnlyPathStillWorks(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"What is the weather in Tokyo?"`),
		Tools: json.RawMessage(`[
			{
				"type": "function",
				"name": "get_weather",
				"description": "Get current weather",
				"parameters": {
					"type": "object",
					"properties": {"city": {"type": "string"}},
					"required": ["city"]
				}
			}
		]`),
	}
	chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 1)
	data, err := json.Marshal(chatReq.Tools)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get current weather",
				"parameters": {
					"type": "object",
					"properties": {"city": {"type": "string"}},
					"required": ["city"]
				}
			}
		}
	]`, string(data))
}

func responsesChatToolNamesFromAny(t *testing.T, tools any) []string {
	t.Helper()
	data, err := json.Marshal(tools)
	require.NoError(t, err)
	var decoded []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	names := make([]string, 0, len(decoded))
	for _, tool := range decoded {
		names = append(names, tool.Function.Name)
	}
	return names
}

// responsesChatAssistantToolCallName extracts the upstream function name of the
// first tool call in the converted chat messages (via RawJSON["messages"]).
func responsesChatAssistantToolCallName(t *testing.T, chatReq *types.ChatCompletionRequest) string {
	t.Helper()
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
	var msgs []struct {
		Role      string `json:"role"`
		ToolCalls []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tool_calls"`
	}
	require.NoError(t, json.Unmarshal(raw["messages"], &msgs))
	for _, msg := range msgs {
		if len(msg.ToolCalls) > 0 {
			return msg.ToolCalls[0].Function.Name
		}
	}
	return ""
}

func TestResponsesNamespacedToolAliasIsDeterministicChatSafeBounded(t *testing.T) {
	short := responsesNamespacedToolAlias("mcp", "read_file")
	require.Equal(t, short, responsesNamespacedToolAlias("mcp", "read_file"))
	require.LessOrEqual(t, len(short), 64)
	require.Regexp(t, `^[a-zA-Z0-9_-]+$`, short)
	require.True(t, strings.HasPrefix(short, "mcp__read_file__"))

	long := responsesNamespacedToolAlias(strings.Repeat("x", 80), strings.Repeat("y", 80))
	require.LessOrEqual(t, len(long), 64)
	require.Regexp(t, `^[a-zA-Z0-9_-]+$`, long)

	// Two identities whose sanitized prefixes coincide (dot/slash collapse to
	// underscore) still get distinct aliases because the hash covers the raw
	// namespace and function.
	require.NotEqual(t,
		responsesNamespacedToolAlias("a/b", "c"),
		responsesNamespacedToolAlias("a_b", "c"))
}

func TestResponsesToChatRequestRejectsDuplicateToolsWithinNamespace(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{
				"type": "namespace",
				"name": "mcp",
				"tools": [
					{"type": "function", "name": "dup", "parameters": {"type": "object"}},
					{"type": "function", "name": "dup", "parameters": {"type": "object"}}
				]
			}
		]`),
	}
	_, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicated tool name within namespace mcp: dup")
}

func TestResponsesToChatRequestRejectsRepeatedTopLevelFunctionTool(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "dup", "parameters": {"type": "object"}},
			{"type": "function", "name": "dup", "parameters": {"type": "object"}}
		]`),
	}
	_, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicated function tool name: dup")
}

func TestResponsesToChatRequestTopLevelAndNamespacedNameStayDistinct(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "read_file", "parameters": {"type": "object"}},
			{
				"type": "namespace",
				"name": "mcp",
				"tools": [
					{"type": "function", "name": "read_file", "parameters": {"type": "object"}}
				]
			}
		]`),
	}
	chatReq, aliases, err := responsesToChatRequestResolved(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 2)

	mcpAlias := responsesNamespacedToolAlias("mcp", "read_file")
	names := responsesChatToolNamesFromAny(t, chatReq.Tools)
	require.ElementsMatch(t, []string{"read_file", mcpAlias}, names)

	// The top-level tool keeps its original name (empty namespace); only the
	// namespaced copy is aliased.
	require.Equal(t, responsesToolIdentity{Name: "read_file"}, aliases.identity("read_file"))
	require.Equal(t, responsesToolIdentity{Name: "read_file", Namespace: "mcp"}, aliases.identity(mcpAlias))
}

func TestResponsesToChatRequestAliasDoesNotCollideWithClientFunctionName(t *testing.T) {
	// A client may legitimately declare a top-level function whose name is exactly
	// the alias of a colliding namespaced identity (aliases use the same charset
	// as client function names). The occupant must be rewritten to its own alias
	// instead of the request being rejected as a "tool alias collision".
	topLevelName := responsesNamespacedToolAlias("mcp", "read_file")
	req := &types.ResponsesRequest{
		Model: "public",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "` + topLevelName + `", "parameters": {"type": "object"}},
			{
				"type": "namespace",
				"name": "mcp",
				"tools": [
					{"type": "function", "name": "read_file", "parameters": {"type": "object"}}
				]
			},
			{
				"type": "namespace",
				"name": "codex",
				"tools": [
					{"type": "function", "name": "read_file", "parameters": {"type": "object"}}
				]
			}
		]`),
	}

	chatReq, aliases, err := responsesToChatRequestResolved(context.Background(), req, "upstream-model", nil)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 3)

	mcpChatName := responsesNamespacedToolAlias("mcp", "read_file") // == topLevelName, claimed by mcp
	codexChatName := responsesNamespacedToolAlias("codex", "read_file")
	topLevelChatName := responsesNamespacedToolAlias("", topLevelName) // the rewritten occupant

	names := responsesChatToolNamesFromAny(t, chatReq.Tools)
	require.ElementsMatch(t, []string{mcpChatName, codexChatName, topLevelChatName}, names)
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		require.False(t, seen[name], "duplicate upstream chat tool name %q", name)
		seen[name] = true
		require.LessOrEqual(t, len(name), 64)
	}

	require.Equal(t, responsesToolIdentity{Name: "read_file", Namespace: "mcp"}, aliases.identity(mcpChatName))
	require.Equal(t, responsesToolIdentity{Name: "read_file", Namespace: "codex"}, aliases.identity(codexChatName))
	require.Equal(t, responsesToolIdentity{Name: topLevelName}, aliases.identity(topLevelChatName))
}

func TestChatResponseToResponsesWithToolAliasesRestoresCollidedIdentities(t *testing.T) {
	mcpAlias := responsesNamespacedToolAlias("mcp", "read_file")
	codexAlias := responsesNamespacedToolAlias("codex", "read_file")
	aliases := newResponsesToolAliases()
	require.NoError(t, aliases.register(responsesToolIdentity{Name: "read_file", Namespace: "mcp"}, mcpAlias))
	require.NoError(t, aliases.register(responsesToolIdentity{Name: "read_file", Namespace: "codex"}, codexAlias))

	resp, err := chatResponseToResponsesWithToolAliases([]byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"upstream-model",
		"choices":[{
			"message":{
				"role":"assistant",
				"tool_calls":[
					{"id":"call_1","type":"function","function":{"name":"`+mcpAlias+`","arguments":"{\"p\":1}"}},
					{"id":"call_2","type":"function","function":{"name":"`+codexAlias+`","arguments":"{\"p\":2}"}}
				]
			},
			"finish_reason":"tool_calls"
		}]
	}`), "public-model", aliases)
	require.NoError(t, err)
	require.Len(t, resp.Output, 2)

	data, err := json.Marshal(resp.Output[0])
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id": "call_1",
		"type": "function_call",
		"status": "completed",
		"call_id": "call_1",
		"name": "read_file",
		"namespace": "mcp",
		"arguments": "{\"p\":1}"
	}`, string(data))

	data, err = json.Marshal(resp.Output[1])
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id": "call_2",
		"type": "function_call",
		"status": "completed",
		"call_id": "call_2",
		"name": "read_file",
		"namespace": "codex",
		"arguments": "{\"p\":2}"
	}`, string(data))
}

func TestResponsesAdapterStreamWriterRestoresCollidedAliasesWithLateName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil, nil, "")

	mcpAlias := responsesNamespacedToolAlias("mcp", "read_file")
	codexAlias := responsesNamespacedToolAlias("codex", "read_file")
	writer.toolAliases = newResponsesToolAliases()
	require.NoError(t, writer.toolAliases.register(responsesToolIdentity{Name: "read_file", Namespace: "mcp"}, mcpAlias))
	require.NoError(t, writer.toolAliases.register(responsesToolIdentity{Name: "read_file", Namespace: "codex"}, codexAlias))
	writer.WriteHeader(200)

	// index 0 resolves immediately: added + first argument delta.
	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"` + mcpAlias + `","arguments":"{\"p\":1}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	// index 1 sends argument fragments before its (aliased) name arrives.
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"arguments":"{\"p\":2}"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	// The alias arrives late: item is added (original identity) then buffered
	// argument fragments flush in order.
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"type":"function","function":{"name":"` + codexAlias + `"}}]}}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	// Raw aliases must never leak to the Responses-visible stream.
	require.NotContains(t, body, mcpAlias)
	require.NotContains(t, body, codexAlias)
	// Both collided identities are restored to their original name + namespace.
	require.Contains(t, body, `"name":"read_file"`)
	mcpAdded := strings.Index(body, `"namespace":"mcp"`)
	codexAdded := strings.Index(body, `"namespace":"codex"`)
	p1Delta := strings.Index(body, `"delta":"{\"p\":1}"`)
	p2Delta := strings.Index(body, `"delta":"{\"p\":2}"`)
	require.NotEqual(t, -1, mcpAdded)
	require.NotEqual(t, -1, codexAdded)
	require.NotEqual(t, -1, p1Delta)
	require.NotEqual(t, -1, p2Delta)
	// index 0 added+delta precede index 1; index 1's item is added before its
	// previously buffered arguments flush.
	require.Less(t, mcpAdded, codexAdded)
	require.Less(t, p1Delta, codexAdded)
	require.Less(t, codexAdded, p2Delta)
	require.Contains(t, body, `"arguments":"{\"p\":1}"`)
	require.Contains(t, body, `"arguments":"{\"p\":2}"`)
}

func TestResponsesToChatRequestHistoryRemapsFunctionCallNames(t *testing.T) {
	collidedTools := json.RawMessage(`[
		{
			"type": "namespace",
			"name": "mcp_filesystem",
			"tools": [
				{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
			]
		},
		{
			"type": "namespace",
			"name": "codex_filesystem",
			"tools": [
				{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
			]
		}
	]`)
	singleTool := json.RawMessage(`[
		{
			"type": "namespace",
			"name": "mcp",
			"tools": [
				{"type": "function", "name": "read_file", "parameters": {"type": "object"}}
			]
		}
	]`)
	mcpAlias := responsesNamespacedToolAlias("mcp_filesystem", "filesystem.read")

	tests := []struct {
		name     string
		tools    json.RawMessage
		input    json.RawMessage
		wantName string
	}{
		{
			name:  "aliased name remapped when namespace present",
			tools: collidedTools,
			input: json.RawMessage(`[
				{"type": "function_call", "call_id": "call_1", "name": "filesystem.read", "arguments": "{}", "namespace": "mcp_filesystem"}
			]`),
			wantName: mcpAlias,
		},
		{
			name:  "ambiguous duplicate name without namespace left unchanged",
			tools: collidedTools,
			input: json.RawMessage(`[
				{"type": "function_call", "call_id": "call_1", "name": "filesystem.read", "arguments": "{}"}
			]`),
			wantName: "filesystem.read",
		},
		{
			name:  "unambiguous namespaced tool keeps its name",
			tools: singleTool,
			input: json.RawMessage(`[
				{"type": "function_call", "call_id": "call_1", "name": "read_file", "arguments": "{}", "namespace": "mcp"}
			]`),
			wantName: "read_file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.ResponsesRequest{
				Model: "public",
				Input: tt.input,
				Tools: tt.tools,
			}
			chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
			require.NoError(t, err)
			require.Equal(t, tt.wantName, responsesChatAssistantToolCallName(t, chatReq))
		})
	}
}

func TestResponsesToChatRequestToolChoiceHandlesCollisions(t *testing.T) {
	collidedTools := json.RawMessage(`[
		{
			"type": "namespace",
			"name": "mcp_filesystem",
			"tools": [
				{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
			]
		},
		{
			"type": "namespace",
			"name": "codex_filesystem",
			"tools": [
				{"type": "function", "name": "filesystem.read", "parameters": {"type": "object"}}
			]
		}
	]`)

	t.Run("collided function name errors", func(t *testing.T) {
		req := &types.ResponsesRequest{
			Model:      "public",
			Input:      json.RawMessage(`"hello"`),
			ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"filesystem.read"}}`),
			Tools:      collidedTools,
		}
		_, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ambiguous tool choice across namespaces: filesystem.read")
	})

	t.Run("unambiguous function name passes through", func(t *testing.T) {
		req := &types.ResponsesRequest{
			Model:      "public",
			Input:      json.RawMessage(`"hello"`),
			ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"read_file"}}`),
			Tools: json.RawMessage(`[
				{
					"type": "namespace",
					"name": "mcp",
					"tools": [
						{"type": "function", "name": "read_file", "parameters": {"type": "object"}}
					]
				}
			]`),
		}
		chatReq, err := responsesToChatRequest(context.Background(), req, "upstream-model", nil)
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		var choice struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		require.NoError(t, json.Unmarshal(raw["tool_choice"], &choice))
		require.Equal(t, "read_file", choice.Function.Name)
	})
}
