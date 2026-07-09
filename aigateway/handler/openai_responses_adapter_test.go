package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestExecuteAdapterResponsesSetsStreamHeader(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"m","stream":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"u","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "adapter-model"},
		InternalModelInfo: types.InternalModelInfo{ImageID: "vllm-gpu"},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       upstream.URL + "/v1/chat/completions",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "adapter-model").Return(model, nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, mock.Anything, model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, mock.Anything, model, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	req := &types.ResponsesRequest{Model: "adapter-model", Input: json.RawMessage(`"hi"`), Stream: true}
	modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream-model", Target: upstream.URL + "/v1/chat/completions", Host: upstream.URL}

	tester.handler.executeAdapterResponses(c, req, modelTarget, "testuuid", "apikey", "adapter-model", nil, nil, nil)

	require.Equal(t, "text/event-stream", c.Writer.Header().Get("Content-Type"))
}

func TestExecuteAdapterResponsesStreamPassthroughUpstreamJSONError(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"m","input":"hi","stream":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "adapter-model"},
		InternalModelInfo: types.InternalModelInfo{ImageID: "vllm-gpu"},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       upstream.URL + "/v1/chat/completions",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, mock.Anything).Return(nil).Once()
	tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, mock.Anything, model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, mock.Anything, model, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	req := &types.ResponsesRequest{Model: "adapter-model", Input: json.RawMessage(`"hi"`), Stream: true}
	modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream-model", Target: upstream.URL + "/v1/chat/completions", Host: upstream.URL}

	tester.handler.executeAdapterResponses(c, req, modelTarget, "testuuid", "apikey", "adapter-model", nil, nil, nil)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), `"code":"rate_limit_exceeded"`)
	require.NotContains(t, w.Body.String(), "response.completed")
	require.NotContains(t, w.Body.String(), "[DONE]")
}
