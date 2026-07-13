package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestOpenAIHandler_Rerank(t *testing.T) {
	t.Run("invalid request body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request.Method = http.MethodPost
		c.Request.Body = http.NoBody

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body, _ := json.Marshal(RerankRequest{
			Model:     "",
			Query:     "what is a panda?",
			Documents: []string{"pandas are bears"},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty query", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body, _ := json.Marshal(RerankRequest{
			Model:     "model1:svc1",
			Query:     "",
			Documents: []string{"pandas are bears"},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty documents", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body, _ := json.Marshal(RerankRequest{
			Model:     "model1:svc1",
			Query:     "what is a panda?",
			Documents: []string{},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body, _ := json.Marshal(RerankRequest{
			Model:     "nonexistent:svc",
			Query:     "what is a panda?",
			Documents: []string{"pandas are bears"},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("get model error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body, _ := json.Marshal(RerankRequest{
			Model:     "model1:svc1",
			Query:     "what is a panda?",
			Documents: []string{"pandas are bears"},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(nil, errors.New("internal error"))

		tester.handler.Rerank(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("successful passthrough with usage capture", func(t *testing.T) {
		tester, c, _ := setupTest(t)

		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// endpoint has no path, the gateway should fall back to /rerank
			require.Equal(t, "/rerank", r.URL.Path)
			var req RerankRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			// model must be rewritten to the resolved model name
			require.Equal(t, "resolved-rerank", req.Model)
			require.Equal(t, "what is a panda?", req.Query)
			require.Equal(t, []string{"pandas are bears", "paris is a city"}, req.Documents)
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"id":"rerank-1","model":"resolved-rerank","usage":{"total_tokens":9},"results":[{"index":0,"relevance_score":0.9}]}`))
			require.NoError(t, err)
		}))
		defer upstream.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{ID: "rerank-model", Object: "model", OwnedBy: "testuser"},
			Upstreams: []commontypes.UpstreamConfig{
				{
					URL:       upstream.URL,
					Enabled:   true,
					ModelName: "resolved-rerank",
				},
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		counter := mocktoken.NewMockEmbeddingTokenCounter(t)
		counter.EXPECT().Input("what is a panda?\npandas are bears\nparis is a city").Return().Once()
		counter.EXPECT().Embedding(mock.MatchedBy(func(usage openai.CreateEmbeddingResponseUsage) bool {
			return usage.PromptTokens == 9 && usage.TotalTokens == 9
		})).Return().Once()
		tester.mocks.tokenCounterFactory.EXPECT().NewEmbedding(token.CreateParam{
			Endpoint: upstream.URL,
			Model:    "resolved-rerank",
		}).Return(counter).Once()
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "rerank-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, "resolved-rerank", counter, "").RunAndReturn(
			func(ctx context.Context, userID string, model *types.Model, targetModelName string, tokenCounter token.Counter, apikey string) error {
				wg.Done()
				return nil
			}).Once()

		body, _ := json.Marshal(RerankRequest{
			Model:     "rerank-model",
			Query:     "what is a panda?",
			Documents: []string{"pandas are bears", "paris is a city"},
		})
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Rerank(c)
		wg.Wait()
	})
}

func TestRerankRequest_JSONRoundTrip(t *testing.T) {
	raw := []byte(`{"model":"m1","query":"q","documents":["d1","d2"],"top_n":2,"return_documents":true,"custom_field":"custom_value"}`)

	var req RerankRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	assert.Equal(t, "m1", req.Model)
	assert.Equal(t, "q", req.Query)
	assert.Equal(t, []string{"d1", "d2"}, req.Documents)
	assert.Equal(t, int64(2), req.TopN)
	require.NotNil(t, req.ReturnDocuments)
	assert.True(t, *req.ReturnDocuments)

	// unknown fields must survive a marshal round trip
	data, err := json.Marshal(req)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "custom_value", out["custom_field"])
}
