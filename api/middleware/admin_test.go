package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// TestPlatformAdminEditionPolicy checks grant mapping against each edition's independent contract.
// Empty requirements are denied by callers before a store query, regardless of their mapped value.
func TestPlatformAdminEditionPolicy(t *testing.T) {
	require.Equal(t, !expectedPlatformAdminGlobalOnly, config.SupportsMultipleAdminRoles())
	for _, tc := range []struct {
		name     string
		required []types.AdminPermission
	}{
		{name: "nil"},
		{name: "empty", required: []types.AdminPermission{}},
		{name: "concrete", required: []types.AdminPermission{types.AdminSystemErrorLog}},
		{name: "multiple", required: []types.AdminPermission{types.AdminSystemErrorLog, types.AdminAccountUser}},
		{name: "membership", required: []types.AdminPermission{types.AdminPermissionAny}},
		{name: "mixed membership", required: []types.AdminPermission{types.AdminSystemErrorLog, types.AdminPermissionAny}},
		{name: "global", required: []types.AdminPermission{types.AdminPermissionAll}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := tc.required
			if expectedPlatformAdminGlobalOnly {
				expected = []types.AdminPermission{types.AdminPermissionAll}
			}
			require.Equal(t, expected, config.PlatformAdminPermissions(tc.required))
		})
	}
}

// TestPlatformAdminAnyRole checks edition-specific administrator access without a business-permission bypass.
func TestPlatformAdminAnyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		username   string
		authType   httpbase.AuthType
		allowed    bool
		checkErr   error
		wantStatus int
	}{
		{name: "anonymous", wantStatus: http.StatusUnauthorized},
		{name: "ordinary user", username: "alice", authType: httpbase.AuthTypeJwt, wantStatus: http.StatusForbidden},
		{name: "administrator with JWT", username: "alice", authType: httpbase.AuthTypeJwt, allowed: true, wantStatus: http.StatusNoContent},
		{name: "administrator with API key", username: "alice", authType: httpbase.AuthTypeUserOrgApiKey, allowed: true, wantStatus: http.StatusNoContent},
		{name: "administrator with access token", username: "alice", authType: httpbase.AuthTypeAccessToken, allowed: true, wantStatus: http.StatusNoContent},
		{name: "system key without user", authType: httpbase.AuthTypeSystemApiKey, wantStatus: http.StatusUnauthorized},
		{name: "database error", username: "alice", checkErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mockdb.NewMockAdminRoleStore(t)
			if tc.username != "" {
				required := types.AdminPermissionAny
				if expectedPlatformAdminGlobalOnly {
					required = types.AdminPermissionAll
				}
				store.EXPECT().HasPermission(mock.Anything, tc.username, required).Return(tc.allowed, tc.checkErr).Once()
			}
			called := false
			engine := gin.New()
			engine.Use(func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, tc.username)
				httpbase.SetAuthType(ctx, tc.authType)
				ctx.Next()
				require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
			})
			engine.GET("/admin/common/version", newPlatformAdmin(store)(types.AdminPermissionAny), func(ctx *gin.Context) {
				called = true
				require.Equal(t, tc.username, httpbase.GetCurrentUser(ctx))
				require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
				ctx.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/admin/common/version?is_admin=true", nil)
			request.Header.Set("X-Admin-Authorized", "true")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, tc.wantStatus, response.Code)
			require.Equal(t, tc.wantStatus == http.StatusNoContent, called)
			if tc.username == "" {
				store.AssertNumberOfCalls(t, "HasPermission", 0)
			}
		})
	}
}

// TestPlatformAdminAuthorization covers identity, denial, error, and success paths.
func TestPlatformAdminAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		authType    httpbase.AuthType
		username    string
		userUUID    string
		allowed     bool
		checkErr    error
		wantStatus  int
		wantChecked bool
	}{
		{name: "JWT allowed", authType: httpbase.AuthTypeJwt, username: "admin", userUUID: "user-uuid", allowed: true, wantStatus: http.StatusOK, wantChecked: true},
		{name: "access token allowed", authType: httpbase.AuthTypeAccessToken, username: "admin", userUUID: "user-uuid", allowed: true, wantStatus: http.StatusOK, wantChecked: true},
		{name: "permission denied", authType: httpbase.AuthTypeJwt, username: "admin", userUUID: "user-uuid", wantStatus: http.StatusForbidden, wantChecked: true},
		{name: "query error", authType: httpbase.AuthTypeJwt, username: "admin", userUUID: "user-uuid", checkErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantChecked: true},
		{name: "missing user", authType: httpbase.AuthTypeJwt, userUUID: "user-uuid", wantStatus: http.StatusUnauthorized},
		{name: "system API key without UUID", authType: httpbase.AuthTypeSystemApiKey, username: "admin", allowed: true, wantStatus: http.StatusOK, wantChecked: true},
		{name: "organization API key", authType: httpbase.AuthTypeUserOrgApiKey, username: "admin", userUUID: "user-uuid", allowed: true, wantStatus: http.StatusOK, wantChecked: true},
		{name: "basic token", username: "admin", userUUID: "user-uuid", allowed: true, wantStatus: http.StatusOK, wantChecked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := mockdb.NewMockAdminRoleStore(t)
			if tt.wantChecked {
				required := types.AdminAccountUser
				if expectedPlatformAdminGlobalOnly {
					required = types.AdminPermissionAll
				}
				checker.EXPECT().HasPermission(mock.Anything, "admin", required).Return(tt.allowed, tt.checkErr)
			}
			router := gin.New()
			handlerCalled := false
			var authorizationPresent bool
			var finalAuthorizationPresent bool
			router.GET("/admin/users", func(ctx *gin.Context) {
				defer func() {
					finalAuthorizationPresent = types.IsAdminAuthorized(ctx.Request.Context())
				}()
				httpbase.SetAuthType(ctx, tt.authType)
				httpbase.SetCurrentUser(ctx, tt.username)
				httpbase.SetCurrentUserUUID(ctx, tt.userUUID)
				ctx.Next()
			}, newPlatformAdmin(checker)(types.AdminAccountUser), func(ctx *gin.Context) {
				handlerCalled = true
				authorizationPresent = types.IsAdminAuthorized(ctx.Request.Context())
				ctx.Status(http.StatusOK)
			})

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
			router.ServeHTTP(response, request)

			require.Equal(t, tt.wantStatus, response.Code)
			if tt.wantChecked {
				checker.AssertNumberOfCalls(t, "HasPermission", 1)
			} else {
				checker.AssertNumberOfCalls(t, "HasPermission", 0)
			}
			require.Equal(t, tt.allowed && tt.checkErr == nil && tt.wantChecked, handlerCalled)
			if handlerCalled {
				require.True(t, authorizationPresent)
				require.True(t, finalAuthorizationPresent)
			} else {
				require.False(t, finalAuthorizationPresent)
			}
		})
	}
}

// TestPlatformAdminAnyPermission verifies edition-specific queries, validation, and authorization markers.
func TestPlatformAdminAnyPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		permissions []types.AdminPermission
		allowed     bool
		checkErr    error
		mutateInput bool
		invalid     bool
		wantStatus  int
	}{
		{name: "token plan", permissions: []types.AdminPermission{types.AdminAPIGatewayTokenPlan}, allowed: true, wantStatus: http.StatusOK},
		{name: "voucher or token plan", permissions: []types.AdminPermission{types.AdminAccountingComputeVoucher, types.AdminAPIGatewayTokenPlan}, allowed: true, wantStatus: http.StatusOK},
		{name: "any permission matches", permissions: []types.AdminPermission{types.AdminSystemDashboard, types.AdminSystemConfig}, allowed: true, wantStatus: http.StatusOK},
		{name: "no permission matches", permissions: []types.AdminPermission{types.AdminSystemDashboard, types.AdminSystemConfig}, wantStatus: http.StatusForbidden},
		{name: "database error", permissions: []types.AdminPermission{types.AdminSystemDashboard, types.AdminSystemConfig}, checkErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
		{name: "invalid permission", permissions: []types.AdminPermission{types.AdminSystemDashboard, ""}, invalid: true, wantStatus: http.StatusInternalServerError},
		{name: "module wildcard requirement is invalid", permissions: []types.AdminPermission{"account/*"}, invalid: true, wantStatus: http.StatusInternalServerError},
		{name: "whitespace requirement is invalid", permissions: []types.AdminPermission{"account/ user"}, invalid: true, wantStatus: http.StatusInternalServerError},
		{name: "global permission requirement", permissions: []types.AdminPermission{types.AdminPermissionAll}, allowed: true, wantStatus: http.StatusOK},
		{name: "no permissions", wantStatus: http.StatusForbidden},
		{name: "empty permissions", permissions: []types.AdminPermission{}, wantStatus: http.StatusForbidden},
		{name: "registration owns permissions", permissions: []types.AdminPermission{types.AdminSystemDashboard, types.AdminSystemConfig}, allowed: true, mutateInput: true, wantStatus: http.StatusOK},
		{name: "any role with concrete permission preserves business authorization", permissions: []types.AdminPermission{types.AdminSystemDashboard, types.AdminPermissionAny}, allowed: true, wantStatus: http.StatusOK},
		{name: "any role with invalid permission fails closed", permissions: []types.AdminPermission{types.AdminPermissionAny, ""}, invalid: true, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := mockdb.NewMockAdminRoleStore(t)
			if len(tt.permissions) > 0 && !tt.invalid {
				required := tt.permissions
				if expectedPlatformAdminGlobalOnly {
					required = []types.AdminPermission{types.AdminPermissionAll}
				}
				permissions := make([]interface{}, len(required))
				for i, permission := range required {
					permissions[i] = permission
				}
				checker.EXPECT().HasPermission(mock.Anything, "admin", permissions...).Return(tt.allowed, tt.checkErr).Once()
			}
			gate := newPlatformAdmin(checker)(tt.permissions...)
			if tt.mutateInput {
				tt.permissions[0] = types.AdminAccountUser
			}
			var handlerCalled, authorized bool
			router := gin.New()
			router.GET("/admin/shared", func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, "admin")
				defer func() { authorized = types.IsAdminAuthorized(ctx.Request.Context()) }()
				ctx.Next()
			}, gate, func(ctx *gin.Context) {
				handlerCalled = true
				ctx.Status(http.StatusOK)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/shared", nil))
			require.Equal(t, tt.wantStatus, response.Code)
			require.Equal(t, tt.wantStatus == http.StatusOK, handlerCalled)
			require.Equal(t, handlerCalled && !slices.Contains(tt.permissions, types.AdminPermissionAny), authorized)
			if len(tt.permissions) == 0 || tt.invalid {
				checker.AssertNumberOfCalls(t, "HasPermission", 0)
			}
		})
	}
}
