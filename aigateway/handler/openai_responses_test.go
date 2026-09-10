package handler

import (
	"testing"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/stretchr/testify/require"
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
