package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// TestRerankPipelineExecute_ProxiesEndpointPathToUpstream pins how the rerank
// proxy (rerank_handler.go) combines the upstream endpoint URL with the client
// request path via resolveProxyPathFromModelEndpoint. Note the rerank-specific
// fallback: when the endpoint has no usable path, the proxy path falls back to
// "/rerank" (not the client path "/v1/rerank").
func TestRerankPipelineExecute_ProxiesEndpointPathToUpstream(t *testing.T) {
	tests := []struct {
		name             string
		endpointSuffix   string
		wantReceivedPath string
	}{
		{
			name:             "full endpoint url with non-v1 prefix forwards endpoint path",
			endpointSuffix:   "/api/v1/rerank",
			wantReceivedPath: "/api/v1/rerank",
		},
		{
			// rerank_handler.go replaces an empty proxy path with "/rerank"
			// instead of preserving the client path.
			name:             "bare host endpoint falls back to /rerank",
			endpointSuffix:   "",
			wantReceivedPath: "/rerank",
		},
		{
			// Documented limitation: a base URL without the terminal segment
			// replaces the client path with its own prefix path. The endpoint
			// must be stored as a full URL.
			name:             "base url with prefix replaces client path",
			endpointSuffix:   "/compatible-mode/v1",
			wantReceivedPath: "/compatible-mode/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester, c, w := setupTest(t)
			tester.mocks.openAIComp.ExpectedCalls = nil

			var receivedPath string
			upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), `"model":"provider-model"`)
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(http.StatusOK)
				_, err = rw.Write([]byte(`{"results":[{"index":0,"relevance_score":0.9}],"model":"provider-model","usage":{"total_tokens":10,"prompt_tokens":8}}`))
				require.NoError(t, err)
			}))
			defer upstream.Close()

			targetURL := upstream.URL + tt.endpointSuffix
			model := &types.Model{
				BaseModel: types.BaseModel{ID: "test-model"},
				Endpoint:  targetURL,
			}
			modelTarget := &types.ModelTarget{
				Model:     model,
				Upstream:  commontypes.UpstreamConfig{ID: 1, URL: targetURL, Enabled: true},
				Target:    targetURL,
				ModelName: "provider-model",
			}

			mockCounter := mocktoken.NewMockEmbeddingTokenCounter(t)
			mockCounter.EXPECT().Input(mock.Anything).Maybe().Return()
			mockCounter.EXPECT().Embedding(mock.Anything).Maybe().Return()
			// Usage returning no usage keeps the async post-process goroutine
			// from touching billing mocks; the path assertions do not depend on it.
			mockCounter.EXPECT().Usage(mock.Anything).Maybe().Return((*token.Usage)(nil), nil)
			tester.mocks.tokenCounterFactory.EXPECT().
				NewEmbedding(token.CreateParam{Endpoint: targetURL, Model: "provider-model"}).
				Return(mockCounter).Once()
			tester.mocks.openAIComp.EXPECT().
				RecordUsageFromTokenUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Maybe().
				Return(nil)

			req := &types.RerankRequest{
				Model:     "provider-model",
				Query:     "hello",
				Documents: []string{"doc-1", "doc-2"},
			}

			c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", bytes.NewReader([]byte(`{"model":"test-model","query":"hello","documents":["doc-1","doc-2"]}`)))
			c.Request.Header.Set("Content-Type", "application/json")

			meta := &types.RequestMetadata{
				Protocol:   string(types.ProtocolChat),
				Task:       "rerank",
				Model:      "test-model",
				TenantID:   "testuuid",
				UserID:     "testuser",
				APIKeyID:   "apikey",
				ParsedBody: req,
			}
			plan := &types.RequestPlan{ModelTarget: modelTarget}

			h := &rerankPipelineHandler{handler: tester.handler}
			err := h.Execute(c, meta, plan)

			require.NoError(t, err)
			require.Equal(t, tt.wantReceivedPath, receivedPath)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}
