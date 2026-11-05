package router

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/handler"
	"testing"
)

// TestDownstreamAdminCancelRoutesAuthorization keeps mirror and multi-source synchronization grants separate.
func TestDownstreamAdminCancelRoutesAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	gate := func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Len(t, required, 1)
		return func(ctx *gin.Context) {
			ctx.Header("X-Required-Permission", string(required[0]))
			if ctx.GetHeader("Authorization") == "" {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			if ctx.GetHeader("X-Test-Grant") != string(required[0]) {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.AbortWithStatus(http.StatusNoContent)
		}
	}
	createAdminRoutes(engine.Group("/api/v1"), gate, &handler.ManagerHandler{})
	for _, route := range []struct {
		method, path string
		permission   types.AdminPermission
	}{
		{http.MethodPost, "/admin/asset/mirror/lfs_sync/cancel", types.AdminAssetMirror},
		{http.MethodPost, "/admin/asset/sync/lfs_sync/cancel", types.AdminAssetSync},
		{http.MethodGet, "/mirror/tasks", types.AdminAssetMirror},
		{http.MethodPost, "/lfs_sync/sync_now", types.AdminAssetMirror},
	} {
		required := route.permission
		for _, tc := range []struct {
			credential, grant string
			status            int
		}{
			{"", string(required), http.StatusUnauthorized},
			{"Bearer original-user", "", http.StatusForbidden},
			{"Bearer original-user", "account/user", http.StatusForbidden},
			{"Bearer original-user", string(required), http.StatusNoContent},
		} {
			t.Run(string(required)+tc.credential+tc.grant, func(t *testing.T) {
				request := httptest.NewRequest(route.method, "/api/v1"+route.path, nil)
				request.Header.Set("Authorization", tc.credential)
				request.Header.Set("X-Test-Grant", tc.grant)
				request.Header.Set("X-Admin", "true")
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				require.Equal(t, tc.status, response.Code)
				require.Equal(t, string(required), response.Header().Get("X-Required-Permission"))
			})
		}
	}
}
