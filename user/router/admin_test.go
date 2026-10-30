package router

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/handler"
)

// downstreamTestGate records route requirements and stops requests before production dependencies.
// PlatformAdmin's database matching and edition policy are covered by middleware tests.
func downstreamTestGate(t *testing.T) func(...types.AdminPermission) gin.HandlerFunc {
	t.Helper()
	return func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.NotEmpty(t, permissions)
		codes := make([]string, len(permissions))
		for i, permission := range permissions {
			codes[i] = string(permission)
		}
		permission := strings.Join(codes, ",")
		return func(ctx *gin.Context) {
			ctx.Header("X-Required-Permission", permission)
			if httpbase.GetCurrentUser(ctx) == "" {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			if !slices.Contains(permissions, types.AdminPermission(ctx.GetHeader("X-Test-Grant"))) {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
			ctx.AbortWithStatus(http.StatusNoContent)
		}
	}
}

// assertDownstreamAdminGate verifies a caller cannot reuse an upstream assertion or an unrelated module grant.
func assertDownstreamAdminGate(t *testing.T, engine *gin.Engine, method, path string, permissions ...types.AdminPermission) {
	t.Helper()
	codes := make([]string, len(permissions))
	for i, permission := range permissions {
		codes[i] = string(permission)
	}
	permission := permissions[0]
	cases := []struct {
		name, username, grant string
		status                int
	}{
		{"anonymous with forged flag", "", string(permission), http.StatusUnauthorized},
		{"ordinary user with forged flag", "alice", "", http.StatusForbidden},
		{"unrelated module", "alice", "system/dashboard", http.StatusForbidden},
		{"matching module", "alice", string(permission), http.StatusNoContent},
	}
	for _, grant := range codes[1:] {
		cases = append(cases, struct {
			name, username, grant string
			status                int
		}{"matching " + grant, "alice", grant, http.StatusNoContent})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(method, path, nil)
			request.Header.Set("X-Test-User", tc.username)
			request.Header.Set("X-Test-Grant", tc.grant)
			request.Header.Set("X-Admin", "true")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, strings.Join(codes, ","), response.Header().Get("X-Required-Permission"))
		})
	}
}

// newDownstreamAdminTestRouter supplies identity without relying on external authentication services.
func newDownstreamAdminTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		httpbase.SetCurrentUser(ctx, ctx.GetHeader("X-Test-User"))
		ctx.Next()
	})
	return engine
}

// TestDownstreamAdminUserRoutesAuthorization checks each shared operation uses its own calling module.
func TestDownstreamAdminUserRoutesAuthorization(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = downstreamTestGate(t)
	group := engine.Group("/api/v1")
	createAdminUserRoutes(group, mc, &handler.UserHandler{})
	createAdminOrganizationVerificationRoutes(group, mc, &handler.OrganizationHandler{})
	createAdminOrganizationCreateRoute(group, mc, func(*gin.Context) { t.Fatal("test gate must stop before creation") })
	for _, tc := range []struct {
		method, path string
		permission   types.AdminPermission
	}{

		{http.MethodDelete, "/user/alice", types.AdminAccountUser},
		{http.MethodPut, "/user/labels", types.AdminAccountUser},
		{http.MethodGet, "/users/stream-export", types.AdminAccountUser},
		{http.MethodPut, "/user/verify/42", types.AdminAccountUser},
		{http.MethodGet, "/admin/account/user/users", types.AdminAccountUser},
		{http.MethodGet, "/admin/account/admin/users/search", types.AdminAccountAdmin},
		{http.MethodGet, "/admin/account/user/user/verify/alice-uuid", types.AdminAccountUser},
		{http.MethodPut, "/organization/verify/42", types.AdminAccountOrganization},
		{http.MethodGet, "/admin/account/organization/organization/verify/org", types.AdminAccountOrganization},
		{http.MethodGet, "/admin/account/user/user/alice", types.AdminAccountUser},
		{http.MethodPut, "/admin/account/user/user/alice-uuid", types.AdminAccountUser},
		{http.MethodGet, "/admin/account/organization/users", types.AdminAccountOrganization},
		{http.MethodGet, "/admin/compute/node/users", types.AdminComputeNode},
		{http.MethodGet, "/admin/asset/model/users", types.AdminAssetModel},
		{http.MethodGet, "/admin/asset/dataset/users", types.AdminAssetDataset},
		{http.MethodGet, "/admin/academy/user_permission/users", types.AdminAcademyUserPermission},
		{http.MethodPost, "/admin/account/organization/organizations", types.AdminAccountOrganization},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) { assertDownstreamAdminGate(t, engine, tc.method, "/api/v1"+tc.path, tc.permission) })
	}
}

// TestDownstreamAdminUserWrappersPreserveRequest verifies aliases retain identity, request context and profile identifiers.
func TestDownstreamAdminUserWrappersPreserveRequest(t *testing.T) {
	for _, method := range []string{"AccountUsers", "AdminCandidateUsers", "OrganizationUsers", "NodeUsers", "ModelUsers", "DatasetUsers", "AcademyUsers", "AccountUser", "UpdateAccountUser", "CreateOrganization"} {
		t.Run(method, func(t *testing.T) {
			engine := newDownstreamAdminTestRouter()
			called := false
			h := &adminUserRouteHandler{target: func(ctx *gin.Context) {
				called = true
				require.Equal(t, "operator", httpbase.GetCurrentUser(ctx))
				require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
				require.Equal(t, "page=2&search=alice&type=uuid", ctx.Request.URL.RawQuery)
				body, err := io.ReadAll(ctx.Request.Body)
				require.NoError(t, err)
				require.Equal(t, `{"name":"Alice"}`, string(body))
				if method == "AccountUser" {
					require.Equal(t, "alice-uuid", ctx.Param("username"))
				}
				if method == "UpdateAccountUser" {
					require.Equal(t, "alice-uuid", ctx.Param("id"))
				}
				ctx.Status(http.StatusNoContent)
			}}
			methods := map[string]gin.HandlerFunc{"AccountUsers": h.AccountUsers, "AdminCandidateUsers": h.AdminCandidateUsers, "OrganizationUsers": h.OrganizationUsers, "NodeUsers": h.NodeUsers, "ModelUsers": h.ModelUsers, "DatasetUsers": h.DatasetUsers, "AcademyUsers": h.AcademyUsers, "AccountUser": h.AccountUser, "UpdateAccountUser": h.UpdateAccountUser, "CreateOrganization": h.CreateOrganization}
			engine.PUT("/user/:identifier", func(ctx *gin.Context) {
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				ctx.Next()
			}, methods[method])
			request := httptest.NewRequest(http.MethodPut, "/user/alice-uuid?page=2&search=alice&type=uuid", strings.NewReader(`{"name":"Alice"}`))
			request.Header.Set("X-Test-User", "operator")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.True(t, called)
			require.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}

// TestDownstreamAdminUserSearchUsesSubmodulePermission prevents cross-submodule access at the user-service boundary.
func TestDownstreamAdminUserSearchUsesSubmodulePermission(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = downstreamTestGate(t)
	createAdminUserRoutes(engine.Group("/api/v1"), mc, &handler.UserHandler{})
	for _, route := range []struct {
		path       string
		permission types.AdminPermission
	}{
		{"/admin/account/user/users", types.AdminAccountUser},
		{"/admin/account/admin/users/search", types.AdminAccountAdmin},
	} {
		assertDownstreamAdminGate(t, engine, http.MethodGet, "/api/v1"+route.path+"?search=alice&page=1&per=20", route.permission)
		otherPermission := types.AdminAccountAdmin
		if route.permission == types.AdminAccountAdmin {
			otherPermission = types.AdminAccountUser
		}
		t.Run(route.path+" rejects the other account grant", func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1"+route.path, nil)
			request.Header.Set("X-Test-User", "manager")
			request.Header.Set("X-Test-Grant", string(otherPermission))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
		})
	}
	// Possessing account/admin must not open account/user profile operations.
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/user/alice"},
		{http.MethodPut, "/user/alice"},
		{http.MethodGet, "/user/verify/alice"},
	} {
		request := httptest.NewRequest(route.method, "/api/v1/admin/account/user"+route.path, nil)
		request.Header.Set("X-Test-User", "manager")
		request.Header.Set("X-Test-Grant", "account/admin")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}

// TestDownstreamAdminOrganizationQueryRoutesAuthorization verifies independent gates and separation from public reads.
func TestDownstreamAdminOrganizationQueryRoutesAuthorization(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = downstreamTestGate(t)
	group := engine.Group("/api/v1")
	createAdminOrganizationQueryRoutes(group, mc, &handler.OrganizationHandler{})
	for _, route := range []struct {
		path       string
		permission types.AdminPermission
	}{
		{"/admin/account/organization/organizations", types.AdminAccountOrganization},
		{"/admin/account/organization/organization/acme", types.AdminAccountOrganization},
		{"/admin/compute/node/organizations", types.AdminComputeNode},
	} {
		t.Run(route.path, func(t *testing.T) {
			assertDownstreamAdminGate(t, engine, http.MethodGet, "/api/v1"+route.path, route.permission)
			assertOrganizationQueryModuleIsolation(t, engine, "/api/v1"+route.path, route.permission)
		})
	}

	// Administrator registration must not attach a gate to the existing public router group.
	publicRead := func(ctx *gin.Context) {
		require.Empty(t, httpbase.GetCurrentUser(ctx))
		require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
		ctx.Status(http.StatusNoContent)
	}
	group.GET("/organizations", publicRead)
	group.GET("/organization/:namespace", publicRead)
	for _, path := range []string{"/organizations", "/organization/acme"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil))
		require.Equal(t, http.StatusNoContent, response.Code)
		require.Empty(t, response.Header().Get("X-Required-Permission"))
	}
}

// assertOrganizationQueryModuleIsolation rejects other organization selector grants even with an upstream context assertion.
func assertOrganizationQueryModuleIsolation(t *testing.T, engine *gin.Engine, path string, permission types.AdminPermission) {
	t.Helper()
	for _, grant := range []types.AdminPermission{types.AdminAccountOrganization, types.AdminComputeNode, types.AdminAccountingComputeVoucher} {
		request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(types.WithAdminAuthorization(context.Background()))
		request.Header.Set("X-Test-User", "operator")
		request.Header.Set("X-Test-Grant", string(grant))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		status := http.StatusForbidden
		if grant == permission {
			status = http.StatusNoContent
		}
		require.Equal(t, status, response.Code, "path %s with grant %s", path, grant)
	}
}

// TestDownstreamAdminOrganizationWrappersPreserveRequest verifies aliases preserve the authorized organization request.
func TestDownstreamAdminOrganizationWrappersPreserveRequest(t *testing.T) {
	for _, wrapper := range []struct {
		name   string
		handle func(*adminUserRouteHandler) gin.HandlerFunc
		detail bool
	}{
		{"AccountOrganizations", func(h *adminUserRouteHandler) gin.HandlerFunc { return h.AccountOrganizations }, false},
		{"AccountOrganization", func(h *adminUserRouteHandler) gin.HandlerFunc { return h.AccountOrganization }, true},
		{"NodeOrganizations", func(h *adminUserRouteHandler) gin.HandlerFunc { return h.NodeOrganizations }, false},
	} {
		t.Run(wrapper.name, func(t *testing.T) { assertOrganizationQueryWrapperPreservesRequest(t, wrapper.handle, wrapper.detail) })
	}
}

// assertOrganizationQueryWrapperPreservesRequest checks path, query, actor and parent context through a wrapper.
func assertOrganizationQueryWrapperPreservesRequest(t *testing.T, wrapper func(*adminUserRouteHandler) gin.HandlerFunc, detail bool) {
	t.Helper()
	engine := newDownstreamAdminTestRouter()
	const query = "page=2&per=20&search=acme&org_type=company&verify_status=approved&tag=AI"
	type requestKey struct{}
	requestContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	t.Cleanup(cancel)
	deadline, _ := requestContext.Deadline()
	requestContext = context.WithValue(requestContext, requestKey{}, "request-value")
	called := false
	h := &adminUserRouteHandler{target: func(ctx *gin.Context) {
		called = true
		require.Equal(t, "operator", httpbase.GetCurrentUser(ctx))
		require.Equal(t, "acme", ctx.Param("name"))
		if detail {
			require.Equal(t, "acme", ctx.Param("namespace"))
		} else {
			require.Empty(t, ctx.Param("namespace"))
		}
		require.Equal(t, query, ctx.Request.URL.RawQuery)
		require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
		require.Equal(t, "request-value", ctx.Request.Context().Value(requestKey{}))
		actualDeadline, ok := ctx.Request.Context().Deadline()
		require.True(t, ok)
		require.Equal(t, deadline, actualDeadline)
		ctx.JSON(http.StatusOK, gin.H{"data": gin.H{"name": "acme"}})
	}}
	engine.GET("/organization/:name", func(ctx *gin.Context) {
		ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
		ctx.Next()
	}, wrapper(h))
	request := httptest.NewRequest(http.MethodGet, "/organization/acme?"+query, nil).WithContext(requestContext)
	request.Header.Set("X-Test-User", "operator")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.True(t, called)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"data":{"name":"acme"}}`, response.Body.String())
}

// TestDownstreamCommonUserRoutesRequireMembership checks local membership, including forged upstream assertions.
// Edition-specific membership policy is covered by the PlatformAdmin middleware tests.
func TestDownstreamCommonUserRoutesRequireMembership(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
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
	createAdminCommonUserRoutes(engine.Group("/api/v1"), mc, &handler.UserHandler{})
	require.Len(t, engine.Routes(), 4)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/admin/common/user/alice"},
		{http.MethodPut, "/admin/common/user/alice-uuid"},
		{http.MethodPost, "/admin/common/user/tags"},
		{http.MethodPost, "/admin/common/user/email-verification-code/alice@example.com"},
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
					request := httptest.NewRequest(route.method, "/api/v1"+route.path, nil)
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

// TestDownstreamCommonUserWrappersPreserveScope checks parameter adapters and original request/response scope.
func TestDownstreamCommonUserWrappersPreserveScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		call      func(*adminUserRouteHandler, *gin.Context)
		parameter string
	}{
		{"profile", (*adminUserRouteHandler).CommonUser, "username"},
		{"update profile", (*adminUserRouteHandler).CommonUpdateUser, "id"},
		{"own tags", (*adminUserRouteHandler).CommonUserTags, ""},
		{"email verification", (*adminUserRouteHandler).CommonEmailVerificationCode, "email"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/?type=uuid&timezone=Asia%2FShanghai", strings.NewReader(`[1,2]`))
			ctx.Request.Header.Set("Authorization", "Bearer original-user")
			ctx.Params = gin.Params{{Key: "identifier", Value: "alice-uuid"}, {Key: "email", Value: "alice@example.com"}}
			httpbase.SetCurrentUser(ctx, "alice")
			httpbase.SetCurrentUserUUID(ctx, "alice-uuid")
			requestContext := ctx.Request.Context()
			calls := 0
			h := &adminUserRouteHandler{target: func(c *gin.Context) {
				calls++
				require.Equal(t, requestContext, c.Request.Context())
				require.False(t, types.IsAdminAuthorized(c.Request.Context()))
				require.Equal(t, "alice", httpbase.GetCurrentUser(c))
				require.Equal(t, "alice-uuid", httpbase.GetCurrentUserUUID(c))
				require.Equal(t, "Bearer original-user", c.GetHeader("Authorization"))
				require.Equal(t, "type=uuid&timezone=Asia%2FShanghai", c.Request.URL.RawQuery)
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, `[1,2]`, string(body))
				if tc.parameter != "" {
					want := "alice-uuid"
					if tc.parameter == "email" {
						want = "alice@example.com"
					}
					require.Equal(t, want, c.Param(tc.parameter))
				}
				c.JSON(http.StatusAccepted, gin.H{"data": "original-response"})
			}}
			tc.call(h, ctx)
			require.Equal(t, 1, calls)
			require.Equal(t, http.StatusAccepted, response.Code)
			require.JSONEq(t, `{"data":"original-response"}`, response.Body.String())
		})
	}
}

// TestDownstreamCommonJWTRequiresMembership rejects requests before administrator JWT processing.
func TestDownstreamCommonJWTRequiresMembership(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = downstreamTestGate(t)
	createAdminCommonJWTRoutes(engine.Group("/api/v1"), mc, &handler.JWTHandler{})
	assertDownstreamAdminGate(t, engine, http.MethodPost, "/api/v1/admin/common/jwt/token", types.AdminPermissionAny)
}

// TestDownstreamNamespaceLookupGate rejects a forwarded admin assertion without the subscription permission.
func TestDownstreamNamespaceLookupGate(t *testing.T) {
	engine := newDownstreamAdminTestRouter()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = downstreamTestGate(t)
	mc.Auth.NeedAdmin = func(ctx *gin.Context) { t.Fatal("lookup must not require legacy global authorization") }
	createAdminNamespaceLookupRoute(engine.Group("/api/v1/admin"), mc, func(ctx *gin.Context) { t.Fatal("gate stops before lookup") })
	assertDownstreamAdminGate(t, engine, http.MethodPost, "/api/v1/admin/namespaces/lookup", types.AdminAPIGatewayTokenPlan)
}
