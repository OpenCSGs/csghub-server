package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestResolvePreviousResponseRoute(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.handler.config.AIGateway.ResponsesIDSecret = "responses-secret"
	mapper, err := tester.handler.getResponsesIDMapper()
	require.NoError(t, err)
	publicID, err := mapper.Wrap(responsesIDClaims{
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
