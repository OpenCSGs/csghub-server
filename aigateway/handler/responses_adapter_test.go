package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	deepSeekMetadata := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled":       true,
					"effort_field":  "",
					"enable_extra":  map[string]any{"thinking": map[string]any{"type": "enabled"}},
					"disable_extra": map[string]any{"thinking": map[string]any{"type": "disabled"}},
				},
			},
		},
	}
	glmMetadata := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled":       true,
					"effort_field":  "reasoning_effort",
					"enable_extra":  map[string]any{"enable_thinking": true},
					"disable_extra": map[string]any{"enable_thinking": false},
				},
			},
		},
	}
	disabledMetadata := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled": false,
				},
			},
		},
	}
	genericMetadata := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled":      true,
					"effort_field": "reasoning_effort",
				},
			},
		},
	}

	t.Run("config absent", func(t *testing.T) {
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, nil, json.RawMessage(`{"effort":"high"}`))
		require.NoError(t, err)
		require.Empty(t, chatReq.RawJSON)
	})

	t.Run("enabled false effort none", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(disabledMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"none"}`))
		require.NoError(t, err)
		require.Empty(t, chatReq.RawJSON)
	})

	t.Run("enabled false effort high", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(disabledMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"high"}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported_feature:reasoning")
	})

	t.Run("enabled true effort none merges disable extra", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(deepSeekMetadata)
		chatReq := &ChatCompletionRequest{}
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
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"low"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"low"`, string(raw["reasoning_effort"]))
		require.Equal(t, `true`, string(raw["enable_thinking"]))
	})

	t.Run("enabled true effort xhigh maps to max", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(glmMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"xhigh"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"max"`, string(raw["reasoning_effort"]))
	})

	t.Run("enabled true effort minimal", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"minimal"}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"minimal"`, string(raw["reasoning_effort"]))
	})

	t.Run("enabled true unknown effort", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":"foobar"}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), `invalid reasoning effort: "foobar"`)
	})

	t.Run("effort normalized case and whitespace", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(genericMetadata)
		chatReq := &ChatCompletionRequest{}
		err := applyAdapterReasoningRequest(ctx, chatReq, cfg, json.RawMessage(`{"effort":" High "}`))
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(chatReq.RawJSON, &raw))
		require.Equal(t, `"high"`, string(raw["reasoning_effort"]))
	})

	t.Run("reasoning object without effort", func(t *testing.T) {
		cfg := loadReasoningRequestConfig(glmMetadata)
		chatReq := &ChatCompletionRequest{}
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

func TestResponsesMalformedJSONReturnsOpenAIError(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":`))
	c.Request.Header.Set("Content-Type", "application/json")

	tester.handler.Responses(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var body struct {
		Error types.Error `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "invalid_request_error", body.Error.Code)
	require.Equal(t, "invalid_request_error", body.Error.Type)
	require.Contains(t, body.Error.Message, "invalid responses request body")
}

func TestResponsesNativeDisablesAcceptEncoding(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream_%t", stream), func(t *testing.T) {
			tester, c, w := setupTest(t)
			tester.mocks.openAIComp.ExpectedCalls = nil
			tester.handler.config.AIGateway.ResponsesIDSecret = "responses-secret"

			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "identity", r.Header.Get("Accept-Encoding"))
				require.Equal(t, "/v1/responses", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("event: response.completed\n" + `data: {"type":"response.completed","response":{"id":"resp_upstream","object":"response","status":"completed"}}` + "\n\n"))
					return
				}
				_, _ = w.Write([]byte(`{"id":"resp_upstream","object":"response","status":"completed"}`))
			}))
			defer upstream.Close()

			model := &types.Model{
				BaseModel: types.BaseModel{ID: "native-model", Object: "model", OwnedBy: "testuser"},
				Upstreams: []commontypes.UpstreamConfig{{
					ID:        7,
					URL:       upstream.URL + "/v1/responses",
					Enabled:   true,
					ModelName: "upstream-model",
					Provider:  "openai",
				}},
			}
			tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "native-model").Return(model, nil).Once()
			tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
			tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, upstream.URL+"/v1/responses").Return(nil).Once()
			var wg sync.WaitGroup
			wg.Add(2)
			tester.mocks.openAIComp.EXPECT().
				CommitUsageLimit(mock.Anything, "testuuid", model, mock.Anything).
				RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
					wg.Done()
					return nil
				}).
				Once()
			tester.mocks.openAIComp.EXPECT().
				RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream-model", mock.MatchedBy(func(usage *token.Usage) bool {
					return usage != nil && usage.PromptTokens > 0
				}), "").
				RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
					wg.Done()
					return nil
				}).
				Once()

			reqBody := fmt.Sprintf(`{"model":"native-model","input":"hello","stream":%t}`, stream)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Accept-Encoding", "gzip")

			tester.handler.Responses(c)
			wg.Wait()

			require.Equal(t, http.StatusOK, w.Code)
		})
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
	chatReq.StreamOptions = &StreamOptions{IncludeUsage: true}

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
	writer := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", counter)
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

func TestResponsesAdapterNonStreamWriterForwardsUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", counter)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
	writer.WriteHeader(200)

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"},"finish_reason":"stop","index":0}]}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte(`data: {"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":48,"completion_tokens":116,"total_tokens":164}}` + "\n\n"))
	require.NoError(t, err)
	writer.finishResponseStream()

	body := w.Body.String()
	require.Contains(t, body, `"usage":{"input_tokens":48,"output_tokens":116,"total_tokens":164}`)
}

func TestResponsesAdapterStreamWriterEmitsToolCallItemBeforeArguments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	require.NotContains(t, body[doneIdx:doneIdx+doneEnd], `"arguments"`)
	require.Contains(t, body[doneIdx:doneIdx+doneEnd], `"item_id":"call_1"`)
	require.Contains(t, body, "event: response.output_item.done")
	require.Contains(t, body, "event: response.completed")
}

func TestResponsesAdapterStreamWriterEmitsRefusalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
	writer := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
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
		CommitUsageLimit(mock.Anything, "testuuid", model, counter).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
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
		}), "api-key").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.recordResponsesUsage(c, counter, "testuuid", modelTarget, "api-key")
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
		CommitUsageLimit(mock.Anything, "testuuid", model, mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
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
		}), "api-key").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.recordResponsesUsage(c, counter, "testuuid", modelTarget, "api-key")
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

func TestResponsesAdapterEndToEndNonFunctionToolsDropped(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(body, &parsed))
		tools, ok := parsed["tools"].([]any)
		require.True(t, ok)
		// Only function tools reach the upstream; code_interpreter is dropped
		require.Len(t, tools, 1)
		tool0 := tools[0].(map[string]any)
		require.Equal(t, "function", tool0["type"])
		fn := tool0["function"].(map[string]any)
		require.Equal(t, "get_weather", fn["name"])

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"id":"chatcmpl_1","created":123,"model":"upstream-model",
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "adapter-tools-model", Object: "model", OwnedBy: "testuser"},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        9,
			URL:       upstream.URL + "/v1/chat/completions",
			Enabled:   true,
			ModelName: "upstream-model",
			Provider:  "openai",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "adapter-tools-model").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, upstream.URL+"/v1/chat/completions").Return(nil).Once()
	var wg sync.WaitGroup
	wg.Add(2)
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimit(mock.Anything, "testuuid", model, mock.Anything).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
			wg.Done()
			return nil
		}).Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream-model", mock.Anything, "").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
			wg.Done()
			return nil
		}).Once()

	reqBody := `{"model":"adapter-tools-model","input":"hello","tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}},{"type":"code_interpreter"}]}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	tester.handler.Responses(c)
	wg.Wait()
	require.Equal(t, http.StatusOK, w.Code)
}
