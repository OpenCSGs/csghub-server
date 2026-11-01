package httpbase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

// TestAuditIdentityContext verifies attribution stays consistent through replacement, clearing and authorization.
func TestAuditIdentityContext(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		set       func(*gin.Context, string)
		get       func(*gin.Context) string
	}{
		{"username", "currentUser", httpbase.SetCurrentUser, httpbase.GetCurrentUser},
		{"UUID", "currentUserUUID", httpbase.SetCurrentUserUUID, httpbase.GetCurrentUserUUID},
		{"token", "accessToken", httpbase.SetAccessToken, httpbase.GetAccessToken},
		{"IP", "ip_address", httpbase.SetIPAddress, httpbase.GetIPAddress},
		{"authentication", "authType", func(ctx *gin.Context, value string) { httpbase.SetAuthType(ctx, httpbase.AuthType(value)) }, func(ctx *gin.Context) string { return string(httpbase.GetAuthType(ctx)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx.Request = httptest.NewRequest(http.MethodGet, "/audit", nil).WithContext(parent)
			tc.set(ctx, "authenticated")
			require.Equal(t, "authenticated", tc.get(ctx))
			require.Equal(t, "authenticated", ctx.Request.Context().Value(tc.key))
			// Authentication alone must not grant administrator authorization.
			require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
			ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
			for _, value := range []string{"replacement", ""} {
				tc.set(ctx, value)
				require.Equal(t, value, tc.get(ctx))
				require.Equal(t, value, ctx.Request.Context().Value(tc.key))
				require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
			}
			cancel()
			require.ErrorIs(t, ctx.Request.Context().Err(), context.Canceled)
		})
	}
}

// TestAuditIdentityWithoutRequest preserves setters used before a request is attached in existing callers.
func TestAuditIdentityWithoutRequest(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	httpbase.SetCurrentUser(ctx, "operator")
	httpbase.SetCurrentUserUUID(ctx, "operator-uuid")
	httpbase.SetAccessToken(ctx, "test-token")
	httpbase.SetIPAddress(ctx, "192.0.2.1")
	httpbase.SetAuthType(ctx, httpbase.AuthTypeJwt)
	require.Equal(t, "operator", httpbase.GetCurrentUser(ctx))
	require.Equal(t, "operator-uuid", httpbase.GetCurrentUserUUID(ctx))
	require.Equal(t, "test-token", httpbase.GetAccessToken(ctx))
	require.Equal(t, "192.0.2.1", httpbase.GetIPAddress(ctx))
	require.Equal(t, httpbase.AuthTypeJwt, httpbase.GetAuthType(ctx))
}
