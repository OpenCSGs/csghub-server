package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestResolvePreviousResponseRoute(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.handler.config.AIGateway.ResponsesIDSecret = "responses-secret"
	mapper, err := tester.handler.getResponsesIDMapper()
	require.NoError(t, err)
	publicID, err := mapper.Wrap(responsespkg.IDClaims{
		NamespaceUUID:      "testuuid",
		UpstreamID:         7,
		UpstreamResponseID: "resp_upstream",
	})
	require.NoError(t, err)

	route, ok := tester.handler.resolvePreviousResponseRoute(c, publicID, "testuuid")

	require.True(t, ok)
	require.Equal(t, int64(7), route.RequiredUpstreamID)
	require.Equal(t, "resp_upstream", route.UpstreamResponseID)
}

func TestResolveResponsesModelTargetMapsUnavailablePreviousRoute(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "public-model"},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       "https://example.com/v1/responses",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "public-model").Return(model, nil).Once()

	modelTarget, ok := tester.handler.resolveResponsesModelTarget(c, "testuser", "public-model", 7)

	require.False(t, ok)
	require.Nil(t, modelTarget)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var body struct {
		Error types.Error `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "response_route_unavailable", body.Error.Code)
}

func TestResolveResponsesModelTargetPreservesOtherErrors(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

	modelTarget, ok := tester.handler.resolveResponsesModelTarget(c, "testuser", "missing-model", 7)

	require.False(t, ok)
	require.Nil(t, modelTarget)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var body struct {
		Error types.Error `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "model_not_found", body.Error.Code)
}

func TestResponsesPreRequestSensitivePromptBlocksStream(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.whitelistRule.ExpectedCalls = nil
	tester.mocks.moderationComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"m","input":"bad prompt","stream":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Sensitive prompt should short-circuit before the proxy runs.
		t.Fatal("upstream should not be called when sensitive content is detected")
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "sensitive-model"},
		ExternalModelInfo: types.ExternalModelInfo{
			NeedSensitiveCheck: true,
		},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       upstream.URL + "/v1/responses",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "m").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(mock.Anything, []string{"sensitive-model"}, "sensitive-model").Return(nil, nil).Once()
	tester.mocks.moderationComp.EXPECT().CheckText(mock.Anything, types.TextModerationRequest{
		Content: "bad prompt",
		Key:     "testuuid:sensitive-model",
		Phase:   types.TextModerationPhasePrompt,
		Mode:    types.TextModerationModeStream,
	}).
		Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

	tester.handler.Responses(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "event: response.completed\n")
	require.Contains(t, body, "data: [DONE]\n\n")
	require.Contains(t, body, responsespkg.BlockedMessage)
}

func TestResponsesPreRequestSensitivePromptBlocksNonStream(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.whitelistRule.ExpectedCalls = nil
	tester.mocks.moderationComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"m","input":"bad prompt"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "sensitive-model"},
		ExternalModelInfo: types.ExternalModelInfo{
			NeedSensitiveCheck: true,
		},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       "http://example.invalid/v1/responses",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "m").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(mock.Anything, []string{"sensitive-model"}, "sensitive-model").Return(nil, nil).Once()
	tester.mocks.moderationComp.EXPECT().CheckText(mock.Anything, types.TextModerationRequest{
		Content: "bad prompt",
		Key:     "testuuid:sensitive-model",
		Phase:   types.TextModerationPhasePrompt,
		Mode:    types.TextModerationModeNonStream,
	}).
		Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

	tester.handler.Responses(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "response", resp["object"])
	require.Equal(t, "completed", resp["status"])
}

func TestResponsesPreRequestSensitivePromptCleanAllowsExecution(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.whitelistRule.ExpectedCalls = nil
	tester.mocks.moderationComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"m","input":"safe prompt"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_upstream","object":"response","status":"completed"}`))
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "sensitive-model"},
		ExternalModelInfo: types.ExternalModelInfo{
			NeedSensitiveCheck: true,
		},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       upstream.URL + "/v1/responses",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "m").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(mock.Anything, []string{"sensitive-model"}, "sensitive-model").Return(nil, nil).Once()
	tester.mocks.moderationComp.EXPECT().CheckText(mock.Anything, types.TextModerationRequest{
		Content: "safe prompt",
		Key:     "testuuid:sensitive-model",
		Phase:   types.TextModerationPhasePrompt,
		Mode:    types.TextModerationModeNonStream,
	}).
		Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, mock.Anything, model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, mock.Anything, model, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	tester.handler.Responses(c)
}

func TestResponsesWhitelistSkipsOutputModeration(t *testing.T) {
	tester, c, w := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.whitelistRule.ExpectedCalls = nil
	tester.mocks.moderationComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"m","input":"safe prompt"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"would otherwise be moderated"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "sensitive-model"},
		ExternalModelInfo: types.ExternalModelInfo{
			NeedSensitiveCheck: true,
		},
		Upstreams: []commontypes.UpstreamConfig{{
			ID:        1,
			URL:       upstream.URL + "/v1/chat/completions",
			Enabled:   true,
			ModelName: "upstream-model",
		}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "m").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(mock.Anything, []string{"sensitive-model"}, "sensitive-model").Return(
		[]database.RepositoryFileCheckRule{{RuleType: database.RuleTypeNamespace, Pattern: "sensitive-model"}}, nil,
	).Once()
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, mock.Anything, model, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, mock.Anything, model, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	tester.handler.Responses(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "would otherwise be moderated")
}
