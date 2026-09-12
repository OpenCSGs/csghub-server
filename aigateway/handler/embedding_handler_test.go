package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// TestEmbeddingPipelineExecute_ProxiesEndpointPathToUpstream pins how the
// embeddings proxy (embedding_handler.go) combines the upstream endpoint URL
// with the client request path via resolveProxyPathFromModelEndpoint. The
// embeddings Execute does not go through the planner's BackendURL, so the
// stored upstream URL is the single source of the forwarded path.
func TestEmbeddingPipelineExecute_ProxiesEndpointPathToUpstream(t *testing.T) {
	tests := []struct {
		name             string
		endpointSuffix   string
		wantReceivedPath string
	}{
		{
			name:             "full endpoint url with non-v1 prefix forwards endpoint path",
			endpointSuffix:   "/compatible-mode/v1/embeddings",
			wantReceivedPath: "/compatible-mode/v1/embeddings",
		},
		{
			name:             "bare host endpoint forwards client path",
			endpointSuffix:   "",
			wantReceivedPath: "/v1/embeddings",
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
				_, err = rw.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"provider-model","usage":{"prompt_tokens":2,"total_tokens":2}}`))
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

			req := &types.EmbeddingRequest{}
			req.Model = "provider-model"
			req.Input = openai.EmbeddingNewParamsInputUnion{OfString: openai.String("hello world")}

			c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte(`{"model":"test-model","input":"hello world"}`)))
			c.Request.Header.Set("Content-Type", "application/json")

			meta := &types.RequestMetadata{
				Protocol:   string(types.ProtocolChat),
				Task:       "embedding",
				Model:      "test-model",
				TenantID:   "testuuid",
				UserID:     "testuser",
				APIKeyID:   "apikey",
				ParsedBody: req,
			}
			plan := &types.RequestPlan{ModelTarget: modelTarget}

			h := &embeddingPipelineHandler{handler: tester.handler}
			err := h.Execute(c, meta, plan)

			require.NoError(t, err)
			require.Equal(t, tt.wantReceivedPath, receivedPath)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}
