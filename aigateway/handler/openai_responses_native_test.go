package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// TestExecuteNativeResponses_ProxiesBackendURLPathToUpstream pins the
// responses-native call site (openai_responses_native.go): unlike the chat,
// embeddings, and rerank call sites it resolves the proxy path from the
// planner's BackendURL (decision.BackendURL), not from Model.Endpoint.
func TestExecuteNativeResponses_ProxiesBackendURLPathToUpstream(t *testing.T) {
	tests := []struct {
		name             string
		backendURLSuffix string
		wantReceivedPath string
	}{
		{
			name:             "full backend url with non-v1 prefix forwards backend path",
			backendURLSuffix: "/compatible-mode/v1/responses",
			wantReceivedPath: "/compatible-mode/v1/responses",
		},
		{
			name:             "bare host backend url forwards client path",
			backendURLSuffix: "",
			wantReceivedPath: "/v1/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester, c, w := setupTest(t)
			tester.mocks.openAIComp.ExpectedCalls = nil
			// The responses ID mapper is built from config; a non-empty secret
			// keeps getResponsesIDMapper working and lets Finalize rewrite the
			// upstream response id.
			tester.handler.config.AIGateway.ResponsesIDSecret = "unit-test-responses-id-secret"

			var receivedPath string
			upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(http.StatusOK)
				_, err := rw.Write([]byte(`{"id":"resp_upstream_1","object":"response","created_at":1700000000,"status":"completed","output":[]}`))
				require.NoError(t, err)
			}))
			defer upstream.Close()

			backendURL := upstream.URL + tt.backendURLSuffix
			modelTarget := &resolvedModelTarget{
				Model: &types.Model{
					BaseModel: types.BaseModel{ID: "test-model"},
					Endpoint:  backendURL,
				},
				Upstream:  commontypes.UpstreamConfig{ID: 1, URL: backendURL, Enabled: true},
				ModelName: "provider-model",
				Target:    backendURL,
			}
			decision := responsespkg.RoutingDecision{
				Mode:       responsespkg.ResponsesModeNative,
				BackendURL: backendURL,
			}
			tester.mocks.openAIComp.EXPECT().
				CheckUsageLimit(mock.Anything, "testuuid", modelTarget.Model, backendURL).
				Return(nil).
				Once()
			// The usage post-process goroutine commits the usage window even
			// for zero-token usage.
			tester.mocks.openAIComp.EXPECT().
				CommitUsageLimitFromUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Maybe().
				Return(nil)
			tester.mocks.openAIComp.EXPECT().
				RecordUsageFromTokenUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Maybe().
				Return(nil)

			req := &types.ResponsesRequest{
				Model: "provider-model",
				Input: []byte(`"hi"`),
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"test-model","input":"hi"}`)))
			c.Request.Header.Set("Content-Type", "application/json")

			h := tester.handler
			h.executeNativeResponses(c, req, modelTarget, decision, "testuuid", "testuuid", "apikey", "test-model", "", nil, nil, nil)

			require.Equal(t, tt.wantReceivedPath, receivedPath)
			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), `"id":`)
		})
	}
}

// TestExecuteNativeResponses_BackendURL404Passthrough pins that a responses
// upstream which does not serve the forwarded backend path answers 404 and the
// native writer passes the upstream error through to the client untouched.
func TestExecuteNativeResponses_BackendURL404Passthrough(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.handler.config.AIGateway.ResponsesIDSecret = "unit-test-responses-id-secret"

	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNotFound)
		_, err := rw.Write([]byte(`{"error":{"message":"not found"}}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	backendURL := upstream.URL + "/compatible-mode/v1/responses"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  backendURL,
		},
		Upstream:  commontypes.UpstreamConfig{ID: 1, URL: backendURL, Enabled: true},
		ModelName: "provider-model",
		Target:    backendURL,
	}
	decision := responsespkg.RoutingDecision{
		Mode:       responsespkg.ResponsesModeNative,
		BackendURL: backendURL,
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "testuuid", modelTarget.Model, backendURL).
		Return(nil).
		Once()
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimitFromUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().
		Return(nil)

	req := &types.ResponsesRequest{
		Model: "provider-model",
		Input: []byte(`"hi"`),
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"test-model","input":"hi"}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	tester.handler.executeNativeResponses(c, req, modelTarget, decision, "testuuid", "testuuid", "apikey", "test-model", "", nil, nil, nil)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not found")
}
