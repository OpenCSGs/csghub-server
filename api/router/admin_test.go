package router

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// TestAdminAccountAdminRoutesAuthorization checks shared route registration, login order, and permission wiring.
// Edition-specific grant policy is covered by middleware tests; this gate isolates the route contract.
func TestAdminAccountAdminRoutesAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := []struct {
		method, path, pattern string
		permission            types.AdminPermission
	}{
		{http.MethodGet, "/roles", "/roles", types.AdminPermissionAny},
		{http.MethodGet, "/me/roles", "/me/roles", types.AdminPermissionAny},
		{http.MethodGet, "/users", "/users", types.AdminAccountAdmin},
		{http.MethodPut, "/users/42/roles/finance_admin", "/users/:user_id/roles/:role_code", types.AdminAccountAdmin},
		{http.MethodDelete, "/users/42/roles/finance_admin", "/users/:user_id/roles/:role_code", types.AdminAccountAdmin},
	}
	for _, principal := range []struct {
		name, username string
		authType       httpbase.AuthType
		allowed        bool
	}{
		{name: "anonymous"},
		{name: "JWT denied by permission gate", username: "member", authType: httpbase.AuthTypeJwt},
		{name: "API key denied by permission gate", username: "member", authType: httpbase.AuthTypeUserOrgApiKey},
		{name: "JWT allowed by permission gate", username: "manager", authType: httpbase.AuthTypeJwt, allowed: true},
		{name: "API key allowed by permission gate", username: "manager", authType: httpbase.AuthTypeUserOrgApiKey, allowed: true},
	} {
		for _, route := range routes {
			t.Run(principal.name+"/"+route.method+route.path, func(t *testing.T) {
				engine := gin.New()
				engine.Use(func(ctx *gin.Context) {
					httpbase.SetCurrentUser(ctx, principal.username)
					httpbase.SetAuthType(ctx, principal.authType)
				})
				loginCalls, permissionCalls := 0, 0
				mustLogin := middleware.MustLogin()
				var mc middleware.MiddlewareCollection
				mc.Auth.NeedLogin = func(ctx *gin.Context) {
					loginCalls++
					mustLogin(ctx)
				}
				mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
					return func(ctx *gin.Context) {
						permissionCalls++
						require.Equal(t, 1, loginCalls)
						require.Equal(t, []types.AdminPermission{route.permission}, permissions)
						if !principal.allowed {
							ctx.AbortWithStatus(http.StatusForbidden)
							return
						}
						// Stop after authorization; handler behavior is covered by handler tests.
						ctx.AbortWithStatus(http.StatusNoContent)
					}
				}
				createAdminAccountAdminRoutes(engine.Group("/api/v1"), mc, &handler.AdminHandler{})
				require.Len(t, engine.Routes(), len(routes)+1)
				requireRoute(t, engine.Routes(), route.method, "/api/v1/admin/account/admin"+route.pattern)
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(route.method, "/api/v1/admin/account/admin"+route.path, nil))
				wantStatus := http.StatusForbidden
				if principal.username == "" {
					wantStatus = http.StatusUnauthorized
					require.Zero(t, permissionCalls)
				} else {
					require.Equal(t, 1, permissionCalls)
					if principal.allowed {
						wantStatus = http.StatusNoContent
					}
				}
				require.Equal(t, 1, loginCalls)
				require.Equal(t, wantStatus, response.Code)
			})
		}
	}
}

// TestAdminCommonRoutesRequireMembership verifies all common routes stop before their business handlers.
func TestAdminCommonRoutesRequireMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")
	apiGroup.Group("/admin", func(ctx *gin.Context) {
		t.Fatal("common routes must not inherit the legacy administrator gate")
	}).GET("/legacy", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminPermissionAny}, permissions)
		return func(ctx *gin.Context) { ctx.AbortWithStatus(http.StatusForbidden) }
	}
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	createAdminCommonReadRoutes(apiGroup, mc, &handler.BroadcastHandler{}, &handler.VersionHandler{})
	createAdminCommonTagRoute(apiGroup, mc, func(ctx *gin.Context) { t.Fatal("denied requests must not reach the tag handler") })
	createAdminCommonUserRoutes(apiGroup, mc, proxy)
	createAdminCommonNotificationRoutes(apiGroup, mc, proxy)
	createAdminCommonJWTRoutes(apiGroup, mc, proxy)
	createAdminCommonOrganizationRoutes(apiGroup, mc, proxy)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/broadcasts/active"},
		{http.MethodGet, "/version"},
		{http.MethodGet, "/organizations/mine"},
		{http.MethodPost, "/jwt/token"},
		{http.MethodGet, "/tags"},
		{http.MethodGet, "/user/uuid-1"},
		{http.MethodPut, "/user/uuid-1"},
		{http.MethodPost, "/user/tags"},
		{http.MethodPost, "/user/email-verification-code/alice@example.com"},
		{http.MethodGet, "/notifications/message-types"},
		{http.MethodGet, "/notifications/setting"},
		{http.MethodPut, "/notifications/setting"},
		{http.MethodGet, "/notifications/count"},
		{http.MethodGet, "/notifications/poll/1"},
		{http.MethodGet, "/notifications"},
		{http.MethodPut, "/notifications/read"},
		{http.MethodDelete, "/notifications"},
	}
	require.Len(t, engine.Routes(), len(routes)+1)
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, "/api/v1/admin/common"+route.path, nil))
			require.Equal(t, http.StatusForbidden, response.Code)
		})
	}
}

// TestAdminCommonTagsPreservesQuery verifies membership permits tag choices with the original filters.
func TestAdminCommonTagsPreservesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminPermissionAny}, permissions)
		return func(ctx *gin.Context) {
			httpbase.SetCurrentUser(ctx, "finance-admin")
			ctx.Next()
		}
	}
	const query = "scope=model&category=industry&built_in=true&search=AI%20tools&per=20&page=2"
	called := false
	createAdminCommonTagRoute(engine.Group("/api/v1"), mc, func(ctx *gin.Context) {
		called = true
		require.Equal(t, query, ctx.Request.URL.RawQuery)
		require.Equal(t, "finance-admin", httpbase.GetCurrentUser(ctx))
		require.False(t, types.IsAdminAuthorized(ctx.Request.Context()))
		ctx.JSON(http.StatusOK, gin.H{"data": []gin.H{{"id": 1, "name": "industry-tag"}}, "total": 1})
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/common/tags?"+query, nil))
	require.True(t, called)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"data":[{"id":1,"name":"industry-tag"}],"total":1}`, response.Body.String())
}

// TestAdminCommonProxyPreservesOriginalRequest checks upstream routing, credentials, parameters, and request bodies.
func TestAdminCommonProxyPreservesOriginalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type receivedRequest struct{ method, path, query, body, authorization string }
	requests := make(chan receivedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		requests <- receivedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body), r.Header.Get("Authorization")}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	proxy, err := handler.NewInternalServiceProxyHandler(upstream.URL)
	require.NoError(t, err)
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminPermissionAny}, permissions)
		return func(ctx *gin.Context) {
			httpbase.SetCurrentUser(ctx, "alice")
			ctx.Next()
			if types.IsAdminAuthorized(ctx.Request.Context()) {
				t.Error("common membership must preserve original business authorization")
			}
		}
	}
	engine := gin.New()
	group := engine.Group("/api/v1")
	createAdminCommonUserRoutes(group, mc, proxy)
	createAdminCommonNotificationRoutes(group, mc, proxy)
	createAdminCommonJWTRoutes(group, mc, proxy)
	createAdminCommonOrganizationRoutes(group, mc, proxy)
	// The reverse proxy needs a real HTTP writer with connection notifications.
	server := httptest.NewServer(engine)
	defer server.Close()
	for _, tc := range []struct{ method, path, query, body string }{
		{http.MethodGet, "/organizations/mine", "page=2", ""},
		{http.MethodPost, "/jwt/token", "", `{"uuid":"uuid-1","old_token":"user-credential"}`},
		{http.MethodGet, "/user/uuid-1", "type=uuid", ""},
		{http.MethodPut, "/user/uuid-1", "type=uuid", `{"nickname":"Alice"}`},
		{http.MethodPost, "/user/tags", "", `[1,2]`},
		{http.MethodPost, "/user/email-verification-code/alice@example.com", "", ""},
		{http.MethodGet, "/notifications/message-types", "", ""},
		{http.MethodGet, "/notifications/setting", "timezone=Asia%2FShanghai", ""},
		{http.MethodPut, "/notifications/setting", "", `{"do_not_disturb":true}`},
		{http.MethodGet, "/notifications/count", "", ""},
		{http.MethodGet, "/notifications/poll/1", "timezone=Asia%2FShanghai", ""},
		{http.MethodGet, "/notifications", "notification_type=system&unread_only=true&page=2&page_size=500", ""},
		{http.MethodPut, "/notifications/read", "", `{"ids":[1,2]}`},
		{http.MethodDelete, "/notifications", "", `{"ids":[3]}`},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			path := "/api/v1/admin/common" + tc.path
			if tc.query != "" {
				path += "?" + tc.query
			}
			request, err := http.NewRequest(tc.method, server.URL+path, strings.NewReader(tc.body))
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer user-credential")
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusNoContent, response.StatusCode)
			select {
			case got := <-requests:
				require.Equal(t, receivedRequest{tc.method, "/api/v1/admin/common" + tc.path, tc.query, tc.body, "Bearer user-credential"}, got)
			default:
				t.Fatal("request did not reach the upstream handler")
			}
		})
	}
}

// TestAdminCommonVersionDelegates verifies successful membership reaches the existing version handler.
func TestAdminCommonVersionDelegates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminPermissionAny}, permissions)
		return func(ctx *gin.Context) { ctx.Next() }
	}
	createAdminCommonReadRoutes(engine.Group("/api/v1"), mc, &handler.BroadcastHandler{}, handler.NewVersionHandler())
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/common/version", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"version":`)
	require.Contains(t, response.Body.String(), `"commit":`)
}

// TestAdminBroadcastRoutesRequirePlatformAdmin verifies all broadcast management operations use their own permission.
func TestAdminBroadcastRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")
	legacyAdminGroup := apiGroup.Group("/admin", func(ctx *gin.Context) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
	})
	legacyAdminGroup.GET("/legacy", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Len(t, permissions, 1)
		permission := permissions[0]
		return func(ctx *gin.Context) {
			ctx.Header("X-Test-Admin-Permission", string(permission))
			ctx.AbortWithStatus(http.StatusForbidden)
		}
	}
	createAdminBroadcastRoutes(apiGroup, middlewareCollection, &handler.BroadcastHandler{})
	apiGroup.GET("/broadcasts/:id", (&handler.BroadcastHandler{}).Show)

	for _, route := range []struct{ method, path, pattern, handlerName string }{
		{http.MethodGet, "/api/v1/admin/broadcasts", "/api/v1/admin/broadcasts", "Broadcasts"},
		{http.MethodPost, "/api/v1/admin/broadcasts", "/api/v1/admin/broadcasts", "CreateBroadcast"},
		{http.MethodGet, "/api/v1/admin/broadcasts/1", "/api/v1/admin/broadcasts/:id", "Broadcast"},
		{http.MethodPut, "/api/v1/admin/broadcasts/1", "/api/v1/admin/broadcasts/:id", "UpdateBroadcast"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			for _, registered := range engine.Routes() {
				if registered.Method == route.method && registered.Path == route.pattern {
					require.True(t, strings.HasSuffix(registered.Handler, "."+route.handlerName+"-fm"), registered.Handler)
				}
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, string(types.AdminSystemBroadcast), response.Header().Get("X-Test-Admin-Permission"))
		})
	}
	for _, route := range engine.Routes() {
		if route.Path == "/api/v1/broadcasts/:id" {
			require.True(t, strings.HasSuffix(route.Handler, ".Show-fm"), route.Handler)
		}
	}
}

// TestAdminBroadcastWrappersPreserveRequest verifies authorization, ID, body and business responses survive delegation.
func TestAdminBroadcastWrappersPreserveRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, operation := range []struct {
		name string
		call func(*adminSharedRouteHandler, *gin.Context)
	}{
		{"list", (*adminSharedRouteHandler).Broadcasts},
		{"create", (*adminSharedRouteHandler).CreateBroadcast},
		{"show", (*adminSharedRouteHandler).Broadcast},
		{"update", (*adminSharedRouteHandler).UpdateBroadcast},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
				response := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(response)
				ctx.Request = httptest.NewRequest(http.MethodPut, "/?status=inactive", strings.NewReader(`{"content":"notice","status":"inactive"}`))
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				ctx.Params = gin.Params{{Key: "id", Value: "7"}}
				httpbase.SetCurrentUser(ctx, "operator")
				request := ctx.Request
				calls := 0
				h := &adminSharedRouteHandler{target: func(actual *gin.Context) {
					calls++
					require.Same(t, ctx, actual)
					require.Same(t, request, actual.Request)
					require.Equal(t, "7", actual.Param("id"))
					require.Equal(t, "operator", httpbase.GetCurrentUser(actual))
					require.True(t, types.IsAdminAuthorized(actual.Request.Context()))
					require.Equal(t, "inactive", actual.Query("status"))
					body, err := io.ReadAll(actual.Request.Body)
					require.NoError(t, err)
					require.JSONEq(t, `{"content":"notice","status":"inactive"}`, string(body))
					actual.JSON(status, gin.H{"msg": "business-response"})
				}}
				operation.call(h, ctx)
				require.Equal(t, 1, calls)
				require.Equal(t, status, response.Code)
				require.JSONEq(t, `{"msg":"business-response"}`, response.Body.String())
			}
		})
	}
}

// TestAdminAccountSharedRoutesRequirePlatformAdmin verifies user and organization aliases use separate grants.
func TestAdminAccountSharedRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")
	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.NotEmpty(t, permissions)
		codes := make([]string, len(permissions))
		for i, permission := range permissions {
			codes[i] = string(permission)
		}
		permission := strings.Join(codes, ",")
		return func(ctx *gin.Context) {
			ctx.Header("X-Test-Admin-Permission", permission)
			ctx.AbortWithStatus(http.StatusForbidden)
		}
	}
	middlewareCollection.License.Check = func(ctx *gin.Context) { ctx.Next() }
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	createAdminAccountUserRoutes(apiGroup, middlewareCollection, proxy)
	createAdminAccountOrganizationRoutes(apiGroup, middlewareCollection, proxy, &handler.TagsHandler{})
	createAdminAccountOrganizationHierarchyRoutes(apiGroup, middlewareCollection, proxy)

	for _, route := range []struct {
		method, path string
		permission   string
	}{
		{http.MethodGet, "/api/v1/admin/account/user/users", string(types.AdminAccountUser)},
		{http.MethodGet, "/api/v1/admin/account/admin/users/search", string(types.AdminAccountAdmin)},
		{http.MethodGet, "/api/v1/admin/account/user/user/alice", string(types.AdminAccountUser)},
		{http.MethodPut, "/api/v1/admin/account/user/user/uuid-1", string(types.AdminAccountUser)},
		{http.MethodGet, "/api/v1/admin/account/user/user/verify/uuid-1", string(types.AdminAccountUser)},
		{http.MethodGet, "/api/v1/admin/account/organization/organizations", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/organizations/mine", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/organization/acme", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/organization/verify/acme", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/organizations/uuid-1/units", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/organizations/uuid-1/members", string(types.AdminAccountOrganization)},
		{http.MethodPost, "/api/v1/admin/account/organization/organizations", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/tags", string(types.AdminAccountOrganization)},
		{http.MethodGet, "/api/v1/admin/account/organization/users", string(types.AdminAccountOrganization)},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, string(route.permission), response.Header().Get("X-Test-Admin-Permission"))
		})
	}
}

// TestAdminAccountExclusiveRoutesRequirePlatformAdmin verifies shared-edition user and organization writes.
func TestAdminAccountExclusiveRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")
	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		require.Len(t, permissions, 1)
		permission := permissions[0]
		return func(ctx *gin.Context) {
			ctx.Header("X-Test-Admin-Permission", string(permission))
			ctx.AbortWithStatus(http.StatusForbidden)
		}
	}
	middlewareCollection.Auth.NeedLogin = func(ctx *gin.Context) { ctx.Next() }
	middlewareCollection.License.Check = func(ctx *gin.Context) { ctx.Next() }
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	createUserRoutes(apiGroup, middlewareCollection, proxy, &handler.UserHandler{})
	createOrgRoutes(apiGroup, middlewareCollection, proxy, &handler.OrganizationHandler{}, &config.Config{})

	for _, route := range []struct {
		method, path string
		permission   types.AdminPermission
	}{
		{http.MethodDelete, "/api/v1/user/alice", types.AdminAccountUser},
		{http.MethodPut, "/api/v1/user/labels", types.AdminAccountUser},
		{http.MethodPut, "/api/v1/user/verify/uuid-1", types.AdminAccountUser},
		{http.MethodPut, "/api/v1/organization/verify/uuid-1", types.AdminAccountOrganization},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, string(route.permission), response.Header().Get("X-Test-Admin-Permission"))
		})
	}
}

// TestAdminAccountProxyPreservesOriginalRequest verifies aliases forward the path, query, method, and body.
func TestAdminAccountProxyPreservesOriginalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type receivedRequest struct {
		method, path, query, body string
	}
	requests := make(chan receivedRequest, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		requests <- receivedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body)}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	proxy, err := handler.NewInternalServiceProxyHandler(upstream.URL)
	require.NoError(t, err)

	engine := gin.New()
	apiGroup := engine.Group("/api/v1")
	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.PlatformAdmin = func(...types.AdminPermission) gin.HandlerFunc {
		return func(ctx *gin.Context) { ctx.Next() }
	}
	middlewareCollection.License.Check = func(ctx *gin.Context) { ctx.Next() }
	createAdminAccountUserRoutes(apiGroup, middlewareCollection, proxy)
	createAdminAccountOrganizationRoutes(apiGroup, middlewareCollection, proxy, &handler.TagsHandler{})
	createAdminSharedUserProxyRoutes(apiGroup, middlewareCollection, proxy)
	createAdminMarketingEmailNotificationRoute(apiGroup, middlewareCollection, proxy)
	server := httptest.NewServer(engine)
	defer server.Close()

	for _, tc := range []struct {
		method, path, body string
		want               receivedRequest
	}{
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/account/user/user/alice?type=uuid",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/account/user/user/alice", "type=uuid", ""},
		},
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/compute/node/users?page=2&search=alice",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/compute/node/users", "page=2&search=alice", ""},
		},
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/account/organization/organizations?page=2&per=20&search=acme&org_type=company&verify_status=verified&tag=ai",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/account/organization/organizations", "page=2&per=20&search=acme&org_type=company&verify_status=verified&tag=ai", ""},
		},
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/account/organization/organization/acme",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/account/organization/organization/acme", "", ""},
		},
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/compute/node/organizations?page=2&per=20&search=acme",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/compute/node/organizations", "page=2&per=20&search=acme", ""},
		},
		{
			method: http.MethodGet,
			path:   "/api/v1/admin/academy/user_permission/users?page=1&per=999",
			want:   receivedRequest{http.MethodGet, "/api/v1/admin/academy/user_permission/users", "page=1&per=999", ""},
		},
		{
			method: http.MethodPost,
			path:   "/api/v1/admin/marketing/email_sending/notifications",
			body:   `{"scenario":"recommendation"}`,
			want:   receivedRequest{http.MethodPost, "/api/v1/admin/marketing/email_sending/notifications", "", `{"scenario":"recommendation"}`},
		},
		{
			method: http.MethodPost,
			path:   "/api/v1/admin/account/organization/organizations?source=admin",
			body:   `{"name":"acme"}`,
			want:   receivedRequest{http.MethodPost, "/api/v1/admin/account/organization/organizations", "source=admin", `{"name":"acme"}`},
		},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request, err := http.NewRequest(tc.method, server.URL+tc.path, strings.NewReader(tc.body))
			require.NoError(t, err)
			response, err := http.DefaultClient.Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusNoContent, response.StatusCode)
			require.Equal(t, tc.want, <-requests)
		})
	}
}

// adminRouteTestMiddleware observes route grants without invoking external business dependencies.
func adminRouteTestMiddleware() middleware.MiddlewareCollection {
	var collection middleware.MiddlewareCollection
	next := func(ctx *gin.Context) { ctx.Next() }
	collection.Auth.NeedLogin = next
	collection.Auth.NeedPhoneVerified = next
	collection.Auth.NeedAPIKey = next
	collection.Auth.NeedAccessToken = next
	collection.Auth.UserMatch = next
	collection.Auth.NeedAdmin = func(ctx *gin.Context) { ctx.AbortWithStatus(http.StatusUnauthorized) }
	collection.Auth.PlatformAdmin = func(permissions ...types.AdminPermission) gin.HandlerFunc {
		return func(ctx *gin.Context) {
			for _, permission := range permissions {
				ctx.Writer.Header().Add("X-Test-Admin-Permission", string(permission))
			}
			if !slices.Contains(permissions, types.AdminPermission(ctx.GetHeader("X-Test-Grant"))) {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			// The production gate is tested separately; stop before business handlers here.
			ctx.AbortWithStatus(http.StatusNoContent)
		}
	}
	collection.License.Check = next
	collection.License.FeatureGate = func(types.FeatureDefinition) gin.HandlerFunc { return next }
	collection.Repo.RepoExists = next
	return collection
}

// requireAdminRoutePermission catches missing gates, wrong submodule grants and inherited legacy gates.
func requireAdminRoutePermission(t *testing.T, engine *gin.Engine, method, path string, permissions ...types.AdminPermission) {
	t.Helper()
	codes := make([]string, len(permissions))
	for i, permission := range permissions {
		codes[i] = string(permission)
	}
	for _, grant := range append([]string{"", "unrelated/submodule"}, codes...) {
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("X-Test-Grant", grant)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		wantStatus := http.StatusForbidden
		if slices.Contains(codes, grant) {
			wantStatus = http.StatusNoContent
		}
		require.Equal(t, wantStatus, response.Code, "%s %s with grant %q", method, path, grant)
		require.Equal(t, codes, response.Header().Values("X-Test-Admin-Permission"), "%s %s", method, path)
	}
}

// TestAdminRemainingSharedRoutesRequirePlatformAdmin checks each registered alias against its owning submodule.
func TestAdminRemainingSharedRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	mc := adminRouteTestMiddleware()
	group.POST("/admin/:repo_type/:namespace/:name/change_path", mc.Auth.NeedAdmin, func(ctx *gin.Context) { t.Fatal("legacy handler must be gated") })
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	repo := &handler.RepoHandler{}
	model := &handler.ModelHandler{}
	tags := &handler.TagsHandler{}
	createAdminSharedResourceRoutes(group, mc, &handler.SpaceResourceHandler{})
	createAdminSharedClusterRoutes(group, mc, &handler.ClusterHandler{})
	createAdminSharedUserProxyRoutes(group, mc, proxy)
	createAdminAssetModelRoutes(group, mc, model, repo)
	createAdminAssetDatasetRoutes(group, mc, &handler.DatasetHandler{}, repo)
	createAdminAssetCodeRoutes(group, mc, &handler.CodeHandler{}, repo)
	createAdminAssetSpaceRoutes(group, mc, &handler.SpaceHandler{}, repo)
	createAdminAssetMCPRoutes(group, mc, &handler.MCPServerHandler{}, repo)
	createAdminAssetSkillRoutes(group, mc, &handler.SkillHandler{}, repo)
	createAdminModelCrossRoutes(group, mc, model, repo)
	createAdminMCPToolsRoute(group, mc, &handler.MCPServerHandler{})
	createAdminAssetTagReadRoutes(group, mc, tags)
	createAdminAssetChangePathRoutes(group, mc, repo)
	createAdminRuntimeFrameworkRoute(group, mc, model)
	createAdminMarketingCodeRoute(group, mc, &handler.CodeHandler{})
	createAdminMarketingEmailNotificationRoute(group, mc, proxy)
	createAdminCommercialAPISelectorRoutes(group, mc, model, tags, &handler.AccountingHandler{})

	// These cases also catch accidentally omitted route registrations.
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/compute/node/cluster"},
		{http.MethodGet, "/api/v1/admin/compute/node/space_resources/hardware_types"},
		{http.MethodGet, "/api/v1/admin/asset/runtime_framework/runtime_framework"},
		{http.MethodGet, "/api/v1/admin/asset/mcp/mcps/tools"},
		{http.MethodPost, "/api/v1/admin/asset/model/models/:namespace/:name/change_path"},
		{http.MethodGet, "/api/v1/admin/academy/user_permission/users"},
		{http.MethodGet, "/api/v1/admin/marketing/eai_article/codes"},
		{http.MethodPost, "/api/v1/admin/marketing/email_sending/notifications"},
	} {
		requireRoute(t, engine.Routes(), route.method, route.path)
	}
	for _, route := range engine.Routes() {
		if strings.Contains(route.Path, ":repo_type") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(route.Path, "/api/v1/admin/"), "/")
		permission := types.AdminPermission(parts[0] + "/" + parts[1])
		permissions := []types.AdminPermission{permission}
		// Only the four recipient selectors are shared with subscription administration.
		switch route.Path {
		case "/api/v1/admin/accounting/compute_voucher/users", "/api/v1/admin/accounting/compute_voucher/organizations", "/api/v1/admin/accounting/compute_voucher/user/:username", "/api/v1/admin/accounting/compute_voucher/organization/:name":
			permissions = append(permissions, types.AdminAPIGatewayTokenPlan)
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			requireAdminRoutePermission(t, engine, route.Method, route.Path, permissions...)
		})
	}
}

// TestAdminRemainingExclusiveRoutesRequirePlatformAdmin checks original URLs without invoking their handlers.
func TestAdminRemainingExclusiveRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	mc := adminRouteTestMiddleware()
	cfg := &config.Config{}
	repo := &handler.RepoHandler{}
	model := &handler.ModelHandler{}
	createTagsRoutes(group, mc, &handler.TagsHandler{})
	createRuntimeFrameworkRoutes(group, mc, model, &handler.RuntimeArchitectureHandler{}, repo)
	createModelRoutes(cfg, group, mc, model, repo, &handler.MonitorHandler{})
	createDatasetRoutes(cfg, group, mc, &handler.DatasetHandler{}, repo)
	createCodeRoutes(cfg, group, mc, &handler.CodeHandler{}, repo)
	createSpaceRoutes(cfg, group, mc, &handler.SpaceHandler{}, repo, &handler.MonitorHandler{})
	createSkillRoutes(cfg, group, mc, &handler.SkillHandler{}, repo)
	CreateMCPServerRoutes(group, mc, &handler.MCPServerHandler{}, repo)
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	createLfsSyncRoutes(group, mc, proxy)
	for _, route := range []struct {
		method, path string
		permission   types.AdminPermission
	}{
		{http.MethodPost, "/api/v1/tags", types.AdminAssetTag},
		{http.MethodGet, "/api/v1/tags/1", types.AdminAssetTag},
		{http.MethodPut, "/api/v1/tags/1", types.AdminAssetTag},
		{http.MethodDelete, "/api/v1/tags/1", types.AdminAssetTag},
		{http.MethodPost, "/api/v1/tags/categories", types.AdminAssetTag},
		{http.MethodPut, "/api/v1/tags/categories/1", types.AdminAssetTag},
		{http.MethodDelete, "/api/v1/tags/categories/1", types.AdminAssetTag},
		{http.MethodPost, "/api/v1/runtime_framework", types.AdminAssetRuntimeFramework},
		{http.MethodPut, "/api/v1/runtime_framework/1", types.AdminAssetRuntimeFramework},
		{http.MethodDelete, "/api/v1/runtime_framework/1", types.AdminAssetRuntimeFramework},
		{http.MethodPost, "/api/v1/runtime_framework/ns/model/scan", types.AdminAssetModel},
		{http.MethodPost, "/api/v1/models/ns/model/serverless", types.AdminAPIGatewayPublicInference},
		{http.MethodDelete, "/api/v1/models/ns/model/serverless", types.AdminAPIGatewayPublicInference},
		{http.MethodGet, "/api/v1/models/ns/model/serverless/1", types.AdminAPIGatewayPublicInference},
		{http.MethodPut, "/api/v1/models/ns/model/serverless/1", types.AdminAPIGatewayPublicInference},
		{http.MethodPut, "/api/v1/models/ns/model/serverless/1/start", types.AdminAPIGatewayPublicInference},
		{http.MethodPut, "/api/v1/models/ns/model/serverless/1/stop", types.AdminAPIGatewayPublicInference},
		{http.MethodGet, "/api/v1/models/ns/model/serverless/1/logs/last", types.AdminAPIGatewayPublicInference},
		{http.MethodPost, "/api/v1/lfs_sync/sync_now", types.AdminAssetMirror},
		{http.MethodPost, "/api/v1/lfs_sync/cancel", types.AdminAssetMirror},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			requireAdminRoutePermission(t, engine, route.method, route.path, route.permission)
		})
	}
}

// mirrorRouteTestResource binds transport names to the repository kinds used by administrator routes.
type mirrorRouteTestResource struct {
	path     string
	repoType types.RepositoryType
}

// mirrorRouteTestResources lists the six repository kinds managed by asset/mirror.
func mirrorRouteTestResources() []mirrorRouteTestResource {
	return []mirrorRouteTestResource{
		{"models", types.ModelRepo}, {"datasets", types.DatasetRepo}, {"codes", types.CodeRepo},
		{"spaces", types.SpaceRepo}, {"mcps", types.MCPServerRepo}, {"skills", types.SkillRepo},
	}
}

// mirrorRouteTestOperation records the independent wrapper and original repository handler for each operation.
type mirrorRouteTestOperation struct {
	method, suffix, wrapperName, handlerName string
	wrapper                                  func(*adminSharedRouteHandler, *gin.Context)
}

// mirrorRouteTestOperations keeps route and wrapper assertions consistent across repository kinds.
func mirrorRouteTestOperations() []mirrorRouteTestOperation {
	return []mirrorRouteTestOperation{
		{http.MethodGet, "/mirror", "AssetMirrorGet", "GetMirror", (*adminSharedRouteHandler).AssetMirrorGet},
		{http.MethodPost, "/mirror", "AssetMirrorCreate", "CreateMirror", (*adminSharedRouteHandler).AssetMirrorCreate},
		{http.MethodPut, "/mirror", "AssetMirrorUpdate", "UpdateMirror", (*adminSharedRouteHandler).AssetMirrorUpdate},
		{http.MethodDelete, "/mirror", "AssetMirrorDelete", "DeleteMirror", (*adminSharedRouteHandler).AssetMirrorDelete},
		{http.MethodPost, "/mirror/sync", "AssetMirrorSync", "SyncMirror", (*adminSharedRouteHandler).AssetMirrorSync},
	}
}

// requireMirrorRouteHandler checks an actual registered route delegates to the expected handler method.
func requireMirrorRouteHandler(t *testing.T, routes gin.RoutesInfo, method, path, handlerName string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			require.True(t, strings.HasSuffix(route.Handler, "."+handlerName+"-fm"), route.Handler)
			return
		}
	}
	require.Failf(t, "missing mirror route", "%s %s", method, path)
}

// registerOrdinaryMirrorRouteTestResources registers the original repository handlers with no external components.
func registerOrdinaryMirrorRouteTestResources(group *gin.RouterGroup, mc middleware.MiddlewareCollection) {
	cfg := &config.Config{}
	repo := &handler.RepoHandler{}
	createModelRoutes(cfg, group, mc, &handler.ModelHandler{}, repo, &handler.MonitorHandler{})
	createDatasetRoutes(cfg, group, mc, &handler.DatasetHandler{}, repo)
	createCodeRoutes(cfg, group, mc, &handler.CodeHandler{}, repo)
	createSpaceRoutes(cfg, group, mc, &handler.SpaceHandler{}, repo, &handler.MonitorHandler{})
	createSkillRoutes(cfg, group, mc, &handler.SkillHandler{}, repo)
	createPromptRoutes(group, mc, &handler.PromptHandler{}, repo)
	CreateMCPServerRoutes(group, mc, &handler.MCPServerHandler{}, repo)
}

// TestRepositoryMirrorRoutesPreserveOrdinaryAuthorization verifies legacy operations retain login and business handlers without an admin marker.
func TestRepositoryMirrorRoutesPreserveOrdinaryAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(setTestUser)
	mc := adminRouteTestMiddleware()
	mc.Auth.NeedLogin = middleware.MustLogin()
	var expectedType types.RepositoryType
	var observedType types.RepositoryType
	var marked bool
	mc.Repo.RepoExists = func(ctx *gin.Context) {
		require.Equal(t, expectedType, common.RepoTypeFromContext(ctx))
		require.Equal(t, "ns", ctx.Param("namespace"))
		require.Equal(t, "repo", ctx.Param("name"))
		// A validation-only terminal fixture lets the real handlers return before their uninitialized components.
		// Login remains the actual route middleware, including the dataset's direct MustLogin calls.
		for i := range ctx.Params {
			if ctx.Params[i].Key == "namespace" {
				ctx.Params[i].Value = ""
			}
		}
		ctx.Next()
		observedType = common.RepoTypeFromContext(ctx)
		marked = types.IsAdminAuthorized(ctx.Request.Context())
	}
	registerOrdinaryMirrorRouteTestResources(engine.Group("/api/v1"), mc)
	resources := []mirrorRouteTestResource{
		{"models", types.ModelRepo}, {"datasets", types.DatasetRepo}, {"codes", types.CodeRepo},
		{"spaces", types.SpaceRepo}, {"skills", types.SkillRepo}, {"prompts", types.PromptRepo},
		{"mcps", types.MCPServerRepo},
	}
	for _, resource := range resources {
		for _, operation := range mirrorRouteTestOperations() {
			if resource.repoType == types.MCPServerRepo && (operation.method == http.MethodPut || operation.method == http.MethodDelete) {
				continue // MCP business routes historically provide only mirror reads, creation and synchronization.
			}
			t.Run(resource.path+"/"+operation.method+operation.suffix, func(t *testing.T) {
				expectedType = resource.repoType
				pattern := "/api/v1/" + resource.path + "/:namespace/:name" + operation.suffix
				requireMirrorRouteHandler(t, engine.Routes(), operation.method, pattern, operation.handlerName)
				for _, username := range []string{"", "ordinary-user"} {
					observedType, marked = types.UnknownRepo, true
					request := httptest.NewRequest(operation.method, "/api/v1/"+resource.path+"/ns/repo"+operation.suffix, strings.NewReader(`{}`))
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("X-Test-User", username)
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					wantStatus := http.StatusUnauthorized
					if username != "" {
						wantStatus = http.StatusBadRequest
					}
					require.Equal(t, wantStatus, response.Code, response.Body.String())
					require.Equal(t, resource.repoType, observedType)
					require.False(t, marked)
					require.Empty(t, response.Header().Get("X-Test-Admin-Permission"))
				}
			})
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		assertNoRoute(t, engine.Routes(), method, "/api/v1/mcps/:namespace/:name/mirror")
	}
}

// mirrorRouteTestMiddleware injects route decisions while checking repository context after authorization.
// Edition-specific grants and production wildcard matching are covered by middleware tests.
func mirrorRouteTestMiddleware(t *testing.T, expectedType *types.RepositoryType) middleware.MiddlewareCollection {
	t.Helper()
	mc := adminRouteTestMiddleware()
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminAssetMirror}, required)
		return func(ctx *gin.Context) {
			if httpbase.GetCurrentUser(ctx) == "" {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			grant := types.AdminPermission(ctx.GetHeader("X-Test-Grant"))
			if grant != types.AdminAssetMirror && grant != "asset/*" && grant != types.AdminPermissionAll {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
			ctx.Next()
		}
	}
	mc.Repo.RepoExists = func(ctx *gin.Context) {
		require.Equal(t, *expectedType, common.RepoTypeFromContext(ctx))
		require.Equal(t, "ns", ctx.Param("namespace"))
		require.Equal(t, "repo", ctx.Param("name"))
		require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
		if ctx.GetHeader("X-Test-Repo-Missing") == "true" {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		ctx.Header("X-Test-Repo-Type", string(common.RepoTypeFromContext(ctx)))
		ctx.AbortWithStatus(http.StatusNoContent)
	}
	return mc
}

// requireMirrorRouteAuthorization exercises every route's denied and permitted grants before business dependencies.
func requireMirrorRouteAuthorization(t *testing.T, engine *gin.Engine, method, path string, repoType types.RepositoryType) {
	t.Helper()
	for _, scenario := range []struct {
		username, grant string
		status          int
	}{
		{"", "*", http.StatusUnauthorized},
		{"super_admin", "", http.StatusForbidden},
		{"operator", string(types.AdminAssetModel), http.StatusForbidden},
		{"operator", string(types.AdminAssetSync), http.StatusForbidden},
		{"operator", string(types.AdminAssetMirror), http.StatusNoContent},
		{"operator", "asset/*", http.StatusNoContent},
		{"operator", "*", http.StatusNoContent},
	} {
		request := httptest.NewRequest(method, path+"?repo_type=dataset&branch=feature%2Fsync", strings.NewReader(`{"repo_type":"dataset","urgent":true}`))
		request.Header.Set("X-Test-User", scenario.username)
		request.Header.Set("X-Test-Grant", scenario.grant)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, scenario.status, response.Code, "%s %s grant %q", method, path, scenario.grant)
		if scenario.status == http.StatusNoContent {
			require.Equal(t, string(repoType), response.Header().Get("X-Test-Repo-Type"))
		}
	}
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("X-Test-User", "operator")
	request.Header.Set("X-Test-Grant", string(types.AdminAssetMirror))
	request.Header.Set("X-Test-Repo-Missing", "true")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

// TestAdminAssetMirrorRoutesRequireOwnPermission checks six repository kinds and every mirror operation use asset/mirror.
func TestAdminAssetMirrorRoutesRequireOwnPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(setTestUser)
	var expectedType types.RepositoryType
	mc := mirrorRouteTestMiddleware(t, &expectedType)
	createAdminAssetMirrorRoutes(engine.Group("/api/v1"), mc, &handler.RepoHandler{})
	require.Len(t, engine.Routes(), 30)
	for _, resource := range mirrorRouteTestResources() {
		for _, operation := range mirrorRouteTestOperations() {
			t.Run(resource.path+"/"+operation.method+operation.suffix, func(t *testing.T) {
				expectedType = resource.repoType
				pattern := "/api/v1/admin/asset/mirror/" + resource.path + "/:namespace/:name" + operation.suffix
				requireMirrorRouteHandler(t, engine.Routes(), operation.method, pattern, operation.wrapperName)
				requireMirrorRouteAuthorization(t, engine, operation.method, "/api/v1/admin/asset/mirror/"+resource.path+"/ns/repo"+operation.suffix, resource.repoType)
			})
		}
	}
}

// TestAdminAssetMirrorRoutesCoexistWithModelAliases prevents duplicate registrations and cross-submodule grants.
func TestAdminAssetMirrorRoutesCoexistWithModelAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	mc := adminRouteTestMiddleware()
	repo := &handler.RepoHandler{}
	createAdminAssetMirrorRoutes(group, mc, repo)
	createAdminModelCrossRoutes(group, mc, &handler.ModelHandler{}, repo)
	for _, operation := range []struct{ method, suffix string }{
		{http.MethodGet, "/mirror"}, {http.MethodPost, "/mirror/sync"},
	} {
		requireAdminRoutePermission(t, engine, operation.method, "/api/v1/admin/asset/model/models/ns/repo"+operation.suffix, types.AdminAssetModel)
		requireAdminRoutePermission(t, engine, operation.method, "/api/v1/admin/asset/mirror/models/ns/repo"+operation.suffix, types.AdminAssetMirror)
	}
}

// TestAdminAssetMirrorWrappersPreserveRepositoryContext verifies wrappers preserve authorization, parameters and request payloads.
func TestAdminAssetMirrorWrappersPreserveRepositoryContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const query = "branch=feature%2Fsync&source=admin&repo_type=dataset"
	const body = `{"repo_type":"dataset","urgent":true}`
	for _, resource := range mirrorRouteTestResources() {
		for _, operation := range mirrorRouteTestOperations() {
			t.Run(resource.path+"/"+operation.method+operation.suffix, func(t *testing.T) {
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(operation.method, "/api/v1/admin/asset/mirror/"+resource.path+"/ns/repo"+operation.suffix+"?"+query, strings.NewReader(body))
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
				ctx.Params = gin.Params{{Key: "namespace", Value: "ns"}, {Key: "name", Value: "repo"}}
				httpbase.SetCurrentUser(ctx, "operator")
				common.SetRepoTypeContext(ctx, resource.repoType)
				request := ctx.Request
				calls := 0
				wrapped := &adminSharedRouteHandler{target: func(actual *gin.Context) {
					calls++
					require.Same(t, ctx, actual)
					require.Same(t, request, actual.Request)
					require.Equal(t, resource.repoType, common.RepoTypeFromContext(actual))
					require.Equal(t, "ns", actual.Param("namespace"))
					require.Equal(t, "repo", actual.Param("name"))
					require.Equal(t, "operator", httpbase.GetCurrentUser(actual))
					require.True(t, types.IsAdminAuthorized(actual.Request.Context()))
					require.Equal(t, query, actual.Request.URL.RawQuery)
					payload, err := io.ReadAll(actual.Request.Body)
					require.NoError(t, err)
					require.Equal(t, body, string(payload))
					actual.JSON(http.StatusOK, gin.H{"data": "unchanged"})
				}}
				operation.wrapper(wrapped, ctx)
				require.Equal(t, 1, calls)
				require.Equal(t, http.StatusOK, ctx.Writer.Status())
			})
		}
	}
}

// TestAdminAssetChangePathPinsRepositoryType prevents request bodies and query strings changing the submodule's resource type.
func TestAdminAssetChangePathPinsRepositoryType(t *testing.T) {
	for _, tc := range []struct{ name, repoType string }{{"model", "models"}, {"dataset", "datasets"}} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			called := false
			h := &adminAssetChangePathHandler{change: func(ctx *gin.Context) {
				called = true
				require.Equal(t, tc.repoType, ctx.Param("repo_type"))
				require.Equal(t, "ns", ctx.Param("namespace"))
				require.Equal(t, "repo", ctx.Param("name"))
				body, err := io.ReadAll(ctx.Request.Body)
				require.NoError(t, err)
				require.JSONEq(t, `{"new_path":"new/repo"}`, string(body))
				ctx.Status(http.StatusNoContent)
			}}
			fn := h.Model
			if tc.name == "dataset" {
				fn = h.Dataset
			}
			engine.POST("/:namespace/:name", fn)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/ns/repo?repo_type=spaces", strings.NewReader(`{"new_path":"new/repo"}`)))
			require.Equal(t, http.StatusNoContent, response.Code)
			require.True(t, called)
		})
	}
}

// TestAdminRecommendationRoutesRequirePlatformAdmin verifies each route fixes its repository type and requires its own asset grant before business logic.
func TestAdminRecommendationRoutesRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := []struct {
		path       string
		permission types.AdminPermission
		repoType   types.RepositoryType
	}{
		{"/api/v1/admin/asset/model/recom/opweight", types.AdminAssetModel, types.ModelRepo},
		{"/api/v1/admin/asset/dataset/recom/opweight", types.AdminAssetDataset, types.DatasetRepo},
		{"/api/v1/admin/asset/code/recom/opweight", types.AdminAssetCode, types.CodeRepo},
		{"/api/v1/admin/asset/space/recom/opweight", types.AdminAssetSpace, types.SpaceRepo},
		{"/api/v1/admin/asset/mcp/recom/opweight", types.AdminAssetMCP, types.MCPServerRepo},
		{"/api/v1/admin/asset/skill/recom/opweight", types.AdminAssetSkill, types.SkillRepo},
	}
	permissions := []types.AdminPermission{
		types.AdminAssetModel, types.AdminAssetDataset, types.AdminAssetCode,
		types.AdminAssetSpace, types.AdminAssetMCP, types.AdminAssetSkill,
	}
	engine := gin.New()
	var mc middleware.MiddlewareCollection
	mc.Auth.NeedAdmin = func(ctx *gin.Context) { t.Fatal("administrator routes must use PlatformAdmin") }
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Len(t, required, 1)
		require.Contains(t, permissions, required[0])
		return func(ctx *gin.Context) {
			if !slices.Contains(required, types.AdminPermission(ctx.GetHeader("X-Test-Grant"))) {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.Next()
		}
	}
	licenseChecked := false
	mc.License.Check = func(ctx *gin.Context) {
		licenseChecked = true
		ctx.Next()
	}
	handlerCalled := false
	boundRepoType := types.UnknownRepo
	var factoryTypes []types.RepositoryType
	createAdminAssetRecommendationRoutes(engine.Group("/api/v1"), mc, func(repoType types.RepositoryType) gin.HandlerFunc {
		factoryTypes = append(factoryTypes, repoType)
		validate := (&handler.RecomHandler{}).SetOpWeight(repoType)
		return func(ctx *gin.Context) {
			handlerCalled = true
			boundRepoType = repoType
			require.True(t, licenseChecked, "the license check must run before business logic")
			validate(ctx)
		}
	})
	require.ElementsMatch(t, []types.RepositoryType{
		types.ModelRepo, types.DatasetRepo, types.CodeRepo, types.SpaceRepo, types.MCPServerRepo, types.SkillRepo,
	}, factoryTypes)
	require.Len(t, engine.Routes(), len(routes))
	handlers := make(map[string]bool)
	for _, registered := range engine.Routes() {
		require.False(t, handlers[registered.Handler], "each submodule must have an independent handler")
		handlers[registered.Handler] = true
	}
	for _, route := range routes {
		requireRoute(t, engine.Routes(), http.MethodPost, route.path)
		for _, permission := range append(permissions, "", types.AdminAccountUser) {
			t.Run(route.path+"/"+string(permission), func(t *testing.T) {
				licenseChecked = false
				handlerCalled = false
				boundRepoType = types.UnknownRepo
				// A missing repo_id exercises shared request validation without invoking its component.
				request := httptest.NewRequest(http.MethodPost, route.path+"?repo_type=prompt", strings.NewReader(`{"weight":50,"repo_type":"prompt"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Test-Grant", string(permission))
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				allowed := route.permission == permission
				if allowed {
					require.Equal(t, http.StatusBadRequest, response.Code)
					require.Contains(t, response.Body.String(), "unmarshal body failed")
					require.Equal(t, route.repoType, boundRepoType, "the route must pin its actual repository type")
				} else {
					require.Equal(t, http.StatusForbidden, response.Code)
					require.Equal(t, types.UnknownRepo, boundRepoType)
				}
				require.Equal(t, allowed, licenseChecked)
				require.Equal(t, allowed, handlerCalled)
			})
		}
	}
	handlerCalled = false
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/recom/opweight", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
	require.False(t, handlerCalled)
}

// TestAcademyForwardRoutesPreserveOrdinaryRequests keeps public and personal calls outside administrator gates.
func TestAcademyForwardRoutesPreserveOrdinaryRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	createAcademyForwardRoutes(engine.Group("/api/v1"), func(ctx *gin.Context) {
		require.Equal(t, "page=2", ctx.Request.URL.RawQuery)
		ctx.Status(http.StatusNoContent)
	})
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/courses"}, {http.MethodGet, "/courses/1"},
		{http.MethodGet, "/collections"}, {http.MethodGet, "/me/courses"},
		{http.MethodPost, "/courses/1/enroll"}, {http.MethodPost, "/courses/1/purchase"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, "/api/v1/academy"+route.path+"?page=2", nil))
			require.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}

// TestAdminAcademyRoutesAuthorization verifies fixed proxy mappings and isolated submodule grants.
func TestAdminAcademyRoutesAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := []struct {
		method, path, target string
		permission           types.AdminPermission
	}{
		{http.MethodGet, "/course/courses", "/api/v1/academy/admin/courses", types.AdminAcademyCourse},
		{http.MethodPost, "/course/courses", "/api/v1/academy/admin/courses", types.AdminAcademyCourse},
		{http.MethodGet, "/course/courses/course-1", "/api/v1/academy/courses/course-1", types.AdminAcademyCourse},
		{http.MethodPut, "/course/courses/course-1", "/api/v1/academy/admin/courses/course-1", types.AdminAcademyCourse},
		{http.MethodDelete, "/course/courses/course-1", "/api/v1/academy/admin/courses/course-1", types.AdminAcademyCourse},
		{http.MethodGet, "/course/collections", "/api/v1/academy/collections", types.AdminAcademyCourse},
		{http.MethodGet, "/collection/collections", "/api/v1/academy/admin/collections", types.AdminAcademyCollection},
		{http.MethodPost, "/collection/collections", "/api/v1/academy/admin/collections", types.AdminAcademyCollection},
		{http.MethodPut, "/collection/collections/collection-1", "/api/v1/academy/admin/collections/collection-1", types.AdminAcademyCollection},
		{http.MethodDelete, "/collection/collections/collection-1", "/api/v1/academy/admin/collections/collection-1", types.AdminAcademyCollection},
		{http.MethodGet, "/user_permission/courses", "/api/v1/academy/admin/courses", types.AdminAcademyUserPermission},
		{http.MethodGet, "/user_permission/course_grants", "/api/v1/academy/admin/users", types.AdminAcademyUserPermission},
		{http.MethodPost, "/user_permission/users/user-uuid/permissions", "/api/v1/academy/admin/users/user-uuid/permissions", types.AdminAcademyUserPermission},
	}
	engine := gin.New()
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Len(t, required, 1)
		return func(ctx *gin.Context) {
			if ctx.GetHeader("X-Test-User") == "" {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			grant := ctx.GetHeader("X-Test-Grant")
			if grant != "*" && grant != string(required[0]) {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
			ctx.Next()
		}
	}
	forwarded := false
	createAdminAcademyRoutes(engine.Group("/api/v1"), mc, func(api string, params ...string) gin.HandlerFunc {
		return func(ctx *gin.Context) {
			forwarded = true
			require.True(t, types.IsAdminAuthorized(ctx.Request.Context()))
			values := make([]any, 0, len(params))
			for _, param := range params {
				values = append(values, ctx.Param(param))
			}
			finalAPI := api
			if len(values) > 0 {
				finalAPI = fmt.Sprintf(api, values...)
			}
			ctx.Header("X-Test-Target", finalAPI)
			require.Equal(t, "page=2&keyword=course", ctx.Request.URL.RawQuery)
			body, err := io.ReadAll(ctx.Request.Body)
			require.NoError(t, err)
			require.Equal(t, `{"course_id":"course-1","grant":true}`, string(body))
			ctx.Status(http.StatusNoContent)
		}
	})
	require.Len(t, engine.Routes(), len(routes))
	for _, route := range routes {
		for _, grant := range []string{"", "*", string(types.AdminAcademyCourse), string(types.AdminAcademyCollection), string(types.AdminAcademyUserPermission), string(types.AdminAssetModel)} {
			for _, loggedIn := range []bool{false, true} {
				t.Run(route.method+route.path+grant+strconv.FormatBool(loggedIn), func(t *testing.T) {
					forwarded = false
					request := httptest.NewRequest(route.method, "/api/v1/admin/academy"+route.path+"?page=2&keyword=course", strings.NewReader(`{"course_id":"course-1","grant":true}`))
					request.Header.Set("X-Test-Grant", grant)
					if loggedIn {
						request.Header.Set("X-Test-User", "manager")
					}
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					status := http.StatusForbidden
					if !loggedIn {
						status = http.StatusUnauthorized
					} else if grant == "*" || grant == string(route.permission) {
						status = http.StatusNoContent
					}
					require.Equal(t, status, response.Code)
					require.Equal(t, status == http.StatusNoContent, forwarded)
					if forwarded {
						require.Equal(t, route.target, response.Header().Get("X-Test-Target"))
					}
				})
			}
		}
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/course/admin/users"}, {http.MethodPost, "/course/users/user/permissions"},
		{http.MethodGet, "/collection/courses"}, {http.MethodDelete, "/user_permission/courses/course-1"},
		{http.MethodPatch, "/course/courses/course-1"}, {http.MethodGet, "/new_submodule/courses"},
	} {
		forwarded = false
		request := httptest.NewRequest(route.method, "/api/v1/admin/academy"+route.path, nil)
		request.Header.Set("X-Test-User", "manager")
		request.Header.Set("X-Test-Grant", "*")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusNotFound, response.Code)
		require.False(t, forwarded)
	}
}

// TestAdminAssetSyncRoutesRequireOwnPermission keeps synchronization and mirror grants independent.
func TestAdminAssetSyncRoutesRequireOwnPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	mc := adminRouteTestMiddleware()
	proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
	require.NoError(t, err)
	createLfsSyncRoutes(group, mc, proxy)
	createAdminAssetSyncExclusiveRoutes(group, mc, &handler.SyncClientSettingHandler{}, &handler.MirrorHandler{})
	unexpected := func(ctx *gin.Context) { t.Fatal("permission checks must stop before business handlers") }
	createAdminAssetSyncRoutes(group, mc, unexpected, unexpected)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/sync/client_setting"},
		{http.MethodPost, "/api/v1/sync/client_setting"},
		{http.MethodGet, "/api/v1/mirror/repos"},
		{http.MethodPost, "/api/v1/admin/asset/sync/lfs_sync/cancel"},
	}
	for _, resource := range []string{"models", "datasets", "codes", "spaces", "mcps", "skills"} {
		routes = append(routes, struct{ method, path string }{
			http.MethodPost, "/api/v1/admin/asset/sync/" + resource + "/ns/repo/mirror/sync",
		})
	}
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			requireAdminRoutePermission(t, engine, route.method, route.path, types.AdminAssetSync)
			request := httptest.NewRequest(route.method, route.path, nil)
			request.Header.Set("X-Test-Grant", string(types.AdminAssetMirror))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
		})
	}
	requireAdminRoutePermission(t, engine, http.MethodPost, "/api/v1/lfs_sync/cancel", types.AdminAssetMirror)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lfs_sync/cancel", nil)
	request.Header.Set("X-Test-Grant", string(types.AdminAssetSync))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
}

// TestAdminAssetSyncExclusiveRoutesPreserveLicense checks settings retain their license gate without inheriting mirror authorization.
func TestAdminAssetSyncExclusiveRoutesPreserveLicense(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	group.Group("/mirror", func(ctx *gin.Context) {
		t.Fatal("synchronization requests must not inherit mirror authorization")
	}).GET("/existing", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminAssetSync}, required)
		return func(ctx *gin.Context) {
			if ctx.FullPath() == "/api/v1/mirror/repos" {
				ctx.AbortWithStatus(http.StatusNoContent)
				return
			}
			ctx.Next()
		}
	}
	licenseCalls := 0
	mc.License.Check = func(ctx *gin.Context) {
		licenseCalls++
		ctx.AbortWithStatus(http.StatusPaymentRequired)
	}
	createAdminAssetSyncExclusiveRoutes(group, mc, &handler.SyncClientSettingHandler{}, &handler.MirrorHandler{})
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/sync/client_setting", nil))
		require.Equal(t, http.StatusPaymentRequired, response.Code, method)
	}
	require.Equal(t, 2, licenseCalls)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/mirror/repos", nil))
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, 2, licenseCalls, "mirror queue reads must preserve their original license behavior")
}

// TestAdminAssetSyncRetryPreservesRepositoryContext checks every repository kind and prevents missing repositories reaching the handler.
func TestAdminAssetSyncRetryPreservesRepositoryContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const query = "branch=dev%2Ffeature&source=sync"
	const body = `{"urgent":true}`
	var expectedType types.RepositoryType
	var calls []string
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminAssetSync}, required)
		return func(ctx *gin.Context) { ctx.Next() }
	}
	mc.Repo.RepoExists = func(ctx *gin.Context) {
		calls = append(calls, "repo_exists")
		require.Equal(t, expectedType, common.RepoTypeFromContext(ctx))
		if ctx.GetHeader("X-Test-Repo-Missing") == "true" {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		ctx.Next()
	}
	engine := gin.New()
	createAdminAssetSyncRoutes(engine.Group("/api/v1"), mc, func(ctx *gin.Context) {
		calls = append(calls, "handler")
		require.Equal(t, expectedType, common.RepoTypeFromContext(ctx))
		require.Equal(t, "ns", ctx.Param("namespace"))
		require.Equal(t, "repo", ctx.Param("name"))
		require.Equal(t, query, ctx.Request.URL.RawQuery)
		payload, err := io.ReadAll(ctx.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(payload))
		ctx.Status(http.StatusNoContent)
	}, func(ctx *gin.Context) { t.Fatal("retry requests must not reach the cancel handler") })
	for _, resource := range []struct {
		path     string
		repoType types.RepositoryType
	}{
		{"models", types.ModelRepo}, {"datasets", types.DatasetRepo}, {"codes", types.CodeRepo},
		{"spaces", types.SpaceRepo}, {"mcps", types.MCPServerRepo}, {"skills", types.SkillRepo},
	} {
		t.Run(resource.path, func(t *testing.T) {
			expectedType = resource.repoType
			for _, missing := range []bool{false, true} {
				calls = nil
				request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/asset/sync/"+resource.path+"/ns/repo/mirror/sync?"+query, strings.NewReader(body))
				if missing {
					request.Header.Set("X-Test-Repo-Missing", "true")
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				if missing {
					require.Equal(t, http.StatusNotFound, response.Code)
					require.Equal(t, []string{"repo_exists"}, calls)
				} else {
					require.Equal(t, http.StatusNoContent, response.Code)
					require.Equal(t, []string{"repo_exists", "handler"}, calls)
				}
			}
		})
	}
}

// TestAdminAssetSyncCancelPreservesOriginalRequest verifies the alias retains the cancellation service contract.
func TestAdminAssetSyncCancelPreservesOriginalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type receivedRequest struct{ method, path, query, body, authorization string }
	requests := make(chan receivedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		requests <- receivedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body), r.Header.Get("Authorization")}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	proxy, err := handler.NewInternalServiceProxyHandler(upstream.URL)
	require.NoError(t, err)
	var mc middleware.MiddlewareCollection
	mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
		require.Equal(t, []types.AdminPermission{types.AdminAssetSync}, required)
		return func(ctx *gin.Context) { ctx.Next() }
	}
	mc.Repo.RepoExists = func(ctx *gin.Context) { t.Error("cancellation must not require a repository path") }
	engine := gin.New()
	createAdminAssetSyncRoutes(engine.Group("/api/v1"), mc,
		func(ctx *gin.Context) { t.Error("cancellation must not invoke retry") },
		proxy.ProxyToApi("/api/v1/admin/asset/sync/lfs_sync/cancel"))
	server := httptest.NewServer(engine)
	defer server.Close()
	const payload = `{"task_id":123}`
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/admin/asset/sync/lfs_sync/cancel?source=sync", strings.NewReader(payload))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer user-credential")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	select {
	case got := <-requests:
		require.Equal(t, receivedRequest{http.MethodPost, "/api/v1/admin/asset/sync/lfs_sync/cancel", "source=sync", payload, "Bearer user-credential"}, got)
	default:
		t.Fatal("cancellation request did not reach the upstream handler")
	}
}

// TestAdminProxyReauthorizesDownstream verifies administrator aliases retain credentials and require a fresh downstream decision.
func TestAdminProxyReauthorizesDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, downstreamAllowed := range []bool{false, true} {
		name := "denied downstream"
		if downstreamAllowed {
			name = "allowed downstream"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, "/api/v1"+r.Header.Get("X-Test-Target"), r.URL.Path)
				// An upstream Go context value cannot arrive over the HTTP boundary.
				require.False(t, types.IsAdminAuthorized(r.Context()))
				require.Equal(t, "Bearer original-user", r.Header.Get("Authorization"))
				require.Equal(t, "source=admin&page=2", r.URL.RawQuery)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, `{"name":"acme","task_id":123}`, string(body))
				if !downstreamAllowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				localContext := types.WithAdminAuthorization(r.Context())
				require.True(t, types.IsAdminAuthorized(localContext))
				w.WriteHeader(http.StatusNoContent)
			}))
			defer downstream.Close()
			proxy, err := handler.NewInternalServiceProxyHandler(downstream.URL)
			require.NoError(t, err)
			var mc middleware.MiddlewareCollection
			mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
				require.NotEmpty(t, required)
				return func(ctx *gin.Context) {
					httpbase.SetCurrentUser(ctx, "operator")
					ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
					ctx.Next()
				}
			}
			mc.License.Check = func(ctx *gin.Context) { ctx.Next() }
			engine := gin.New()
			group := engine.Group("/api/v1")
			createAdminAccountUserRoutes(group, mc, proxy)
			createAdminAccountOrganizationRoutes(group, mc, proxy, &handler.TagsHandler{})
			createAdminAccountOrganizationHierarchyRoutes(group, mc, proxy)
			createAdminSharedUserProxyRoutes(group, mc, proxy)
			createLfsSyncRoutes(group, mc, proxy)
			createAdminAssetSyncRoutes(group, mc, func(*gin.Context) { t.Fatal("unexpected retry") }, proxy.ProxyToApi("/api/v1/admin/asset/sync/lfs_sync/cancel"))
			server := httptest.NewServer(engine)
			defer server.Close()
			paths := []struct{ method, path, target string }{
				{http.MethodGet, "/admin/account/user/users", "/admin/account/user/users"},
				{http.MethodGet, "/admin/account/admin/users/search", "/admin/account/admin/users/search"},
				{http.MethodGet, "/admin/account/user/user/verify/alice-uuid", "/admin/account/user/user/verify/alice-uuid"},
				{http.MethodGet, "/admin/account/organization/organization/verify/acme", "/admin/account/organization/organization/verify/acme"},
				{http.MethodGet, "/admin/account/user/user/alice", "/admin/account/user/user/alice"},
				{http.MethodPut, "/admin/account/user/user/alice-uuid", "/admin/account/user/user/alice-uuid"},
				{http.MethodPost, "/admin/account/organization/organizations", "/admin/account/organization/organizations"},
				{http.MethodGet, "/admin/account/organization/organizations", "/admin/account/organization/organizations"},
				{http.MethodGet, "/admin/account/organization/organization/acme", "/admin/account/organization/organization/acme"},
				{http.MethodGet, "/admin/account/organization/organizations/mine", "/admin/account/organization/organizations/mine"},
				{http.MethodGet, "/admin/account/organization/organizations/root/units", "/admin/account/organization/organizations/root/units"},
				{http.MethodGet, "/admin/account/organization/organizations/root/members", "/admin/account/organization/organizations/root/members"},
				{http.MethodGet, "/admin/account/organization/users", "/admin/account/organization/users"},
				{http.MethodGet, "/admin/compute/node/users", "/admin/compute/node/users"},
				{http.MethodGet, "/admin/compute/node/organizations", "/admin/compute/node/organizations"},
				{http.MethodGet, "/admin/asset/model/users", "/admin/asset/model/users"},
				{http.MethodGet, "/admin/asset/dataset/users", "/admin/asset/dataset/users"},
				{http.MethodGet, "/admin/academy/user_permission/users", "/admin/academy/user_permission/users"},
				{http.MethodPost, "/lfs_sync/cancel", "/admin/asset/mirror/lfs_sync/cancel"},
				{http.MethodPost, "/admin/asset/sync/lfs_sync/cancel", "/admin/asset/sync/lfs_sync/cancel"},
			}
			// Billing aliases are registered only in SaaS; include them when present.
			for _, route := range engine.Routes() {
				if route.Path == "/api/v1/admin/accounting/compute_voucher/users" {
					paths = append(paths,
						struct{ method, path, target string }{http.MethodGet, "/admin/accounting/compute_voucher/users", "/admin/accounting/compute_voucher/users"},
						struct{ method, path, target string }{http.MethodGet, "/admin/accounting/compute_voucher/user/alice", "/admin/accounting/compute_voucher/user/alice"},
						struct{ method, path, target string }{http.MethodGet, "/admin/accounting/compute_voucher/organizations", "/admin/accounting/compute_voucher/organizations"},
						struct{ method, path, target string }{http.MethodGet, "/admin/accounting/compute_voucher/organization/acme", "/admin/accounting/compute_voucher/organization/acme"},
					)
					break
				}
			}
			wantStatus := http.StatusForbidden
			if downstreamAllowed {
				wantStatus = http.StatusNoContent
			}
			for _, tc := range paths {
				t.Run(tc.method+tc.path, func(t *testing.T) {
					request, err := http.NewRequest(tc.method, server.URL+"/api/v1"+tc.path+"?source=admin&page=2", strings.NewReader(`{"name":"acme","task_id":123}`))
					require.NoError(t, err)
					request.Header.Set("Authorization", "Bearer original-user")
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("X-Test-Target", tc.target)
					response, err := http.DefaultClient.Do(request)
					require.NoError(t, err)
					defer response.Body.Close()
					require.Equal(t, wantStatus, response.StatusCode)
				})
			}
			require.Equal(t, len(paths), calls)
		})
	}
}

// TestPermissionCheckRouteUsesLoginOnly keeps permission evaluation in the handler, including for non-admin users.
func TestPermissionCheckRouteUsesLoginOnly(t *testing.T) {
	for _, user := range []string{"", "member", "operator"} {
		t.Run(user, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { httpbase.SetCurrentUser(c, user) })
			var mc middleware.MiddlewareCollection
			mc.Auth.NeedLogin = func(c *gin.Context) {
				middleware.MustLogin()(c)
				if !c.IsAborted() {
					c.AbortWithStatus(http.StatusNoContent)
				}
			}
			mc.Auth.PlatformAdmin = func(...types.AdminPermission) gin.HandlerFunc {
				return func(c *gin.Context) { t.Fatal("permission-check route must not execute a static administrator gate") }
			}
			createAdminAccountAdminRoutes(router.Group("/api/v1"), mc, &handler.AdminHandler{})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/admin/account/admin/me/permissions/check", nil))
			if user == "" {
				require.Equal(t, 401, response.Code)
			} else {
				require.Equal(t, 204, response.Code)
			}
		})
	}
}

// TestAdminUserSearchUsesSubmodulePermission separates user management from administrator candidates.
// PlatformAdmin's wildcard and edition mapping remain covered by middleware tests.
func TestAdminUserSearchUsesSubmodulePermission(t *testing.T) {
	for _, tc := range []struct {
		name, username, grant, method, path string
		status                              int
	}{
		{"anonymous user list", "", "account/user", http.MethodGet, "/user/users", http.StatusUnauthorized},
		{"anonymous candidates", "", "account/admin", http.MethodGet, "/admin/users/search", http.StatusUnauthorized},
		{"ordinary user list", "member", "", http.MethodGet, "/user/users", http.StatusForbidden},
		{"ordinary candidates", "member", "", http.MethodGet, "/admin/users/search", http.StatusForbidden},
		{"unrelated user list", "manager", "account/organization", http.MethodGet, "/user/users", http.StatusForbidden},
		{"unrelated candidates", "manager", "account/organization", http.MethodGet, "/admin/users/search", http.StatusForbidden},
		{"user manager lists users", "manager", "account/user", http.MethodGet, "/user/users", http.StatusNoContent},
		{"administrator manager searches candidates", "manager", "account/admin", http.MethodGet, "/admin/users/search", http.StatusNoContent},
		{"user manager cannot search administrator candidates", "manager", "account/user", http.MethodGet, "/admin/users/search", http.StatusForbidden},
		{"administrator manager cannot list managed users", "manager", "account/admin", http.MethodGet, "/user/users", http.StatusForbidden},
		{"administrator manager cannot read profiles", "manager", "account/admin", http.MethodGet, "/user/user/alice", http.StatusForbidden},
		{"administrator manager cannot update profiles", "manager", "account/admin", http.MethodPut, "/user/user/alice", http.StatusForbidden},
		{"administrator manager cannot read verification", "manager", "account/admin", http.MethodGet, "/user/user/verify/alice", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			var mc middleware.MiddlewareCollection
			mc.Auth.PlatformAdmin = func(required ...types.AdminPermission) gin.HandlerFunc {
				return func(ctx *gin.Context) {
					want := []types.AdminPermission{types.AdminAccountUser}
					if ctx.FullPath() == "/api/v1/admin/account/admin/users/search" {
						want = []types.AdminPermission{types.AdminAccountAdmin}
					}
					require.Equal(t, want, required)
					if tc.username == "" {
						ctx.AbortWithStatus(http.StatusUnauthorized)
					} else if !slices.Contains(required, types.AdminPermission(tc.grant)) {
						ctx.AbortWithStatus(http.StatusForbidden)
					} else {
						ctx.AbortWithStatus(http.StatusNoContent)
					}
				}
			}
			proxy, err := handler.NewInternalServiceProxyHandler("http://example.invalid")
			require.NoError(t, err)
			createAdminAccountUserRoutes(engine.Group("/api/v1"), mc, proxy)
			// Register role-management routes alongside search to catch static/wildcard collisions.
			mc.Auth.NeedLogin = func(ctx *gin.Context) { ctx.Next() }
			createAdminAccountAdminRoutes(engine.Group("/api/v1"), mc, &handler.AdminHandler{})
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(tc.method, "/api/v1/admin/account"+tc.path, nil))
			require.Equal(t, tc.status, response.Code)
		})
	}
}

// TestNamespaceLookupUsesTokenPlanGate proves the lookup does not inherit the legacy global administrator requirement.
func TestNamespaceLookupUsesTokenPlanGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	mc := adminRouteTestMiddleware()
	legacy := group.Group("/admin", mc.Auth.NeedAdmin)
	legacy.GET("/legacy-only", func(ctx *gin.Context) { t.Fatal("legacy routes retain their global gate") })
	createUserRoutes(group, mc, &handler.InternalServiceProxyHandler{}, &handler.UserHandler{})
	requireAdminRoutePermission(t, engine, http.MethodPost, "/api/v1/admin/namespaces/lookup", types.AdminAPIGatewayTokenPlan)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/legacy-only", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}
