package rpc

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestParaviewGetUserInfoDoesNotLogCredentialOrResponseBody(t *testing.T) {
	const secret = "paraview-access-secret"
	const endpoint = "http://paraview.test"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/esc-sso/oauth2.0/profile?access_token="+secret,
		httpmock.NewStringResponder(http.StatusUnauthorized, `{"access_token":"`+secret+`"}`))

	var logs bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	client := &paraviewClientImpl{endpoint: endpoint}
	_, err := client.GetUserInfo(context.Background(), secret)

	require.Error(t, err)
	require.NotContains(t, logs.String(), secret)
	require.Contains(t, logs.String(), "status_code=401")
}
