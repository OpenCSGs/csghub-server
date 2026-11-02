package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"opencsg.com/csghub-server/common/errorx"
)

func TestCasdoorTokenFailureDoesNotExposeCredentials(t *testing.T) {
	for _, response := range []struct {
		name   string
		status int
		body   string
	}{
		{"error response", http.StatusBadRequest, `{"error":"invalid_grant","error_description":"upstream-secret"}`},
		{"JSON body", http.StatusBadRequest, `{"access_token":"upstream-secret"}`},
		{"SDK error token", http.StatusOK, `{"access_token":"error: upstream-secret","token_type":"Bearer"}`},
	} {
		t.Run(response.name, func(t *testing.T) {
			var logs bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(original) })
			httpmock.RegisterResponder(http.MethodPost, "https://casdoor.test/api/login/oauth/access_token", httpmock.NewStringResponder(response.status, response.body))
			client := NewCasdoorClient(&casdoorsdk.AuthConfig{Endpoint: "https://casdoor.test", ClientId: "client", ClientSecret: "client-secret"}, 50)
			_, err := client.GetOAuthToken(context.Background(), "code-secret", "state-secret")
			require.ErrorIs(t, err, errorx.ErrRemoteServiceFail)
			for _, secret := range []string{"code-secret", "state-secret", "upstream-secret", "client-secret"} {
				require.NotContains(t, logs.String()+err.Error(), secret)
			}
			require.Contains(t, logs.String(), "code_fingerprint")
			if response.status == http.StatusBadRequest {
				var retrieveErr *oauth2.RetrieveError
				require.ErrorAs(t, err, &retrieveErr)
			}
		})
	}
}

func TestCasdoorOAuthErrorPreservesCause(t *testing.T) {
	err := &casdoorOAuthError{cause: context.DeadlineExceeded}
	require.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestCasdoorCreateInvitation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		quota       int
		remoteError bool
	}{
		{name: "default quota", quota: 50},
		{name: "custom quota", quota: 100},
		{name: "remote failure", quota: 100, remoteError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			httpmock.RegisterResponder(http.MethodPost, "https://casdoor.test/api/add-invitation", func(r *http.Request) (*http.Response, error) {
				called = true
				var invitation casdoorsdk.Invitation
				require.NoError(t, json.NewDecoder(r.Body).Decode(&invitation))
				require.Equal(t, tc.quota, invitation.Quota)
				require.Equal(t, "TESTCODE", invitation.Code)
				require.Equal(t, "invitation-TESTCODE", invitation.Name)
				require.Equal(t, "test-app", invitation.Application)
				if tc.remoteError {
					return nil, errors.New("casdoor unavailable")
				}
				return httpmock.NewStringResponse(http.StatusOK, `{"status":"ok","data":"Affected"}`), nil
			})
			client := NewCasdoorClient(&casdoorsdk.AuthConfig{Endpoint: "https://casdoor.test", ApplicationName: "test-app"}, tc.quota)
			err := client.CreateInvitation(context.Background(), "TESTCODE")
			if tc.remoteError {
				require.ErrorIs(t, err, errorx.ErrRemoteServiceFail)
			} else {
				require.NoError(t, err)
			}
			require.True(t, called)
		})
	}
}

func TestCasdoorTokenSuccessPreservesToken(t *testing.T) {
	httpmock.RegisterResponder(http.MethodPost, "https://casdoor.test/api/login/oauth/access_token", httpmock.NewStringResponder(http.StatusOK, `{"access_token":"success-secret","token_type":"Bearer"}`))
	client := NewCasdoorClient(&casdoorsdk.AuthConfig{Endpoint: "https://casdoor.test", ClientId: "client", ClientSecret: "client-secret"}, 50)
	token, err := client.GetOAuthToken(context.Background(), "code-secret", "state-secret")
	require.NoError(t, err)
	require.Equal(t, "success-secret", token.AccessToken)
}
