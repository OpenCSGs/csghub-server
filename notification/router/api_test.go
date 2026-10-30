package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/handler"
)

// TestMarketingNotificationAdminRoute rejects forged assertions and unrelated module grants.
func TestMarketingNotificationAdminRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	gate := func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminMarketingEmailSending}, permissions)
		return func(c *gin.Context) {
			if c.GetHeader("Authorization") == "" {
				c.AbortWithStatus(401)
				return
			}
			if c.GetHeader("Grant") != string(permissions[0]) {
				c.AbortWithStatus(403)
				return
			}
			c.AbortWithStatus(204)
		}
	}
	createAdminRoutes(engine.Group("/api/v1"), gate, &handler.NotificationHandler{})
	for _, tc := range []struct {
		auth, grant string
		status      int
	}{{"", "marketing/email_sending", 401}, {"Bearer user", "", 403}, {"Bearer user", "account/user", 403}, {"Bearer user", "marketing/email_sending", 204}} {
		req := httptest.NewRequest("POST", "/api/v1/admin/marketing/email_sending/notifications", nil)
		req.Header.Set("Authorization", tc.auth)
		req.Header.Set("Grant", tc.grant)
		req.Header.Set("X-Admin", "true")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, tc.status, rec.Code)
	}
}

// TestCommonNotificationRoutesRequireMembership checks each personal entry independently verifies membership.
// The fixture isolates route wiring; edition-specific database matching is tested in middleware.
func TestCommonNotificationRoutesRequireMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		httpbase.SetCurrentUser(ctx, ctx.GetHeader("X-Test-User"))
		ctx.Next()
	})
	gate := func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminPermissionAny}, permissions)
		return func(ctx *gin.Context) {
			if httpbase.GetCurrentUser(ctx) == "" {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			if ctx.GetHeader("X-Test-Local-Membership") != "valid" {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
			ctx.AbortWithStatus(http.StatusNoContent)
		}
	}
	createAdminCommonNotificationRoutes(engine.Group("/api/v1"), gate, &handler.NotificationHandler{})
	require.Len(t, engine.Routes(), 8)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/message-types"},
		{http.MethodGet, "/setting"},
		{http.MethodPut, "/setting"},
		{http.MethodGet, "/count"},
		{http.MethodGet, "/poll/1"},
		{http.MethodGet, ""},
		{http.MethodPut, "/read"},
		{http.MethodDelete, ""},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			for _, principal := range []struct {
				name, username, membership string
				status                     int
			}{
				{"anonymous with forged admin flag", "", "valid", http.StatusUnauthorized},
				{"ordinary user with forged admin flag", "alice", "", http.StatusForbidden},
				{"revoked administrator", "alice", "revoked", http.StatusForbidden},
				{"valid local administrator", "alice", "valid", http.StatusNoContent},
			} {
				t.Run(principal.name, func(t *testing.T) {
					request := httptest.NewRequest(route.method, "/api/v1/admin/common/notifications"+route.path, nil)
					request.Header.Set("X-Test-User", principal.username)
					request.Header.Set("X-Test-Local-Membership", principal.membership)
					request.Header.Set("X-Admin", "true")
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					require.Equal(t, principal.status, response.Code)
				})
			}
		})
	}
}

// TestCommonNotificationWrappersPreserveScope keeps personal identity, request context and responses unchanged.
func TestCommonNotificationWrappersPreserveScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*adminNotificationHandler, *gin.Context)
	}{
		{"message types", (*adminNotificationHandler).CommonNotificationTypes},
		{"settings", (*adminNotificationHandler).CommonNotificationSetting},
		{"update settings", (*adminNotificationHandler).CommonUpdateNotificationSetting},
		{"count", (*adminNotificationHandler).CommonNotificationCount},
		{"poll", (*adminNotificationHandler).CommonNotificationPoll},
		{"list", (*adminNotificationHandler).CommonNotifications},
		{"read", (*adminNotificationHandler).CommonReadNotifications},
		{"delete", (*adminNotificationHandler).CommonDeleteNotifications},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/?page=2&page_size=10&timezone=Asia%2FShanghai", strings.NewReader(`{"ids":[1,2]}`))
			ctx.Request.Header.Set("Authorization", "Bearer original-user")
			ctx.Params = gin.Params{{Key: "limit", Value: "3"}}
			httpbase.SetCurrentUser(ctx, "alice")
			httpbase.SetCurrentUserUUID(ctx, "alice-uuid")
			requestContext := ctx.Request.Context()
			calls := 0
			h := &adminNotificationHandler{target: func(c *gin.Context) {
				calls++
				require.Equal(t, requestContext, c.Request.Context())
				require.False(t, types.IsAdminAuthorized(c.Request.Context()))
				require.Equal(t, "alice", httpbase.GetCurrentUser(c))
				require.Equal(t, "alice-uuid", httpbase.GetCurrentUserUUID(c))
				require.Equal(t, "Bearer original-user", c.GetHeader("Authorization"))
				require.Equal(t, "page=2&page_size=10&timezone=Asia%2FShanghai", c.Request.URL.RawQuery)
				require.Equal(t, "3", c.Param("limit"))
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, `{"ids":[1,2]}`, string(body))
				c.JSON(http.StatusAccepted, gin.H{"data": "original-response"})
			}}
			tc.call(h, ctx)
			require.Equal(t, 1, calls)
			require.Equal(t, http.StatusAccepted, response.Code)
			require.JSONEq(t, `{"data":"original-response"}`, response.Body.String())
		})
	}
}
