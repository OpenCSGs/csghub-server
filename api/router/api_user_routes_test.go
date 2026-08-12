package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/errorx"
)

// needAdminMock returns a test version of NeedAdmin middleware that does not
// require a real user-service RPC. It behaves identically to the real
// middleware.NeedAdmin:
//   - empty currentUser → 401 Unauthorized
//   - non-admin user    → 403 Forbidden
//   - admin user        → next()
func needAdminMock() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser := httpbase.GetCurrentUser(c)
		if currentUser == "" {
			httpbase.UnauthorizedError(c, errorx.ErrUserNotFound)
			c.Abort()
			return
		}
		if currentUser != "admin" {
			httpbase.ForbiddenError(c, errorx.ErrUserNotAdmin)
			c.Abort()
			return
		}
		c.Next()
	}
}

// setTestUser is a middleware that reads the X-Test-User header and sets it as
// the current user in the gin context. This avoids having to inject the user
// between route registration and request handling.
func setTestUser(c *gin.Context) {
	if user := c.GetHeader("X-Test-User"); user != "" {
		httpbase.SetCurrentUser(c, user)
	}
	c.Next()
}

// newUserRoutesTestRouter creates a gin router with the user routes registered
// exactly as in production, backed by a local test HTTP server so the proxy
// returns 200 on success.
func newUserRoutesTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	// Start a local backend that the proxy handler will forward requests to.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	proxyHandler, err := handler.NewInternalServiceProxyHandler(backend.URL)
	if err != nil {
		t.Fatalf("failed to create proxy handler: %v", err)
	}

	mc := middleware.MiddlewareCollection{}
	mc.Auth.NeedLogin = middleware.MustLogin()
	mc.Auth.NeedAdmin = needAdminMock()

	router := gin.New()
	router.Use(setTestUser)

	apiGroup := router.Group("/api/v1")
	createUserRoutes(apiGroup, mc, proxyHandler, &handler.UserHandler{})

	return router
}

func TestUserRoutes_AnonymousAccess_Returns401(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/v1/user/testuser"},
		{http.MethodDelete, "/api/v1/user/testuser"},
		{http.MethodDelete, "/api/v1/user/testuser/close_account"},
		{http.MethodGet, "/api/v1/user/testuser/tokens"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusUnauthorized, resp.Code,
				"anonymous %s %s should return 401", tc.method, tc.path)
		})
	}
}

func TestUserRoutes_NormalUser_DeleteUser_Returns403(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/testuser", nil)
	req.Header.Set("X-Test-User", "normaluser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code,
		"normal user DELETE /user/:username should return 403 (NeedAdmin)")
}

func TestUserRoutes_Admin_DeleteUser_PassesThrough(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/testuser", nil)
	req.Header.Set("X-Test-User", "admin")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"admin DELETE /user/:username should reach the proxy handler")
}

func TestUserRoutes_LoginUser_CloseAccount_PassesThrough(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/testuser/close_account", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user DELETE close_account should reach the proxy (ownership validated downstream)")
}

func TestUserRoutes_LoginUser_PutUser_PassesThrough(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/testuser", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user PUT /user/:username should reach the proxy")
}

func TestUserRoutes_LoginUser_GetTokens_PassesThrough(t *testing.T) {
	router := newUserRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/testuser/tokens", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user GET tokens should reach the proxy (ownership validated downstream)")
}

// =============================================================================
// Token write routes (POST/PUT/DELETE /token/:app/:token_name)
// =============================================================================

func newTokenRoutesTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	proxyHandler, err := handler.NewInternalServiceProxyHandler(backend.URL)
	if err != nil {
		t.Fatalf("failed to create proxy handler: %v", err)
	}

	mc := middleware.MiddlewareCollection{}
	mc.Auth.NeedLogin = middleware.MustLogin()

	router := gin.New()
	router.Use(setTestUser)

	apiGroup := router.Group("/api/v1")
	createTokenRoutes(apiGroup, mc, proxyHandler)

	return router
}

func TestTokenRoutes_AnonymousWriteAccess_Returns401(t *testing.T) {
	router := newTokenRoutesTestRouter(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/token/git/my-token"},
		{http.MethodPut, "/api/v1/token/git/my-token"},
		{http.MethodDelete, "/api/v1/token/git/my-token"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusUnauthorized, resp.Code,
				"anonymous %s %s should return 401", tc.method, tc.path)
		})
	}
}

func TestTokenRoutes_VerifyToken_NoAuthRequired(t *testing.T) {
	router := newTokenRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/token/some-token-value", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"GET /token/:token_value (token verify) should NOT require login")
}

func TestTokenRoutes_LoginUser_CreateToken_PassesThrough(t *testing.T) {
	router := newTokenRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/token/git/my-token", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user POST token should reach the proxy")
}

// =============================================================================
// Organization & member write routes
// =============================================================================

func newOrgRoutesTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	proxyHandler, err := handler.NewInternalServiceProxyHandler(backend.URL)
	if err != nil {
		t.Fatalf("failed to create proxy handler: %v", err)
	}

	mc := middleware.MiddlewareCollection{}
	mc.Auth.NeedLogin = middleware.MustLogin()
	mc.Auth.NeedAdmin = needAdminMock()

	router := gin.New()
	router.Use(setTestUser)

	apiGroup := router.Group("/api/v1")
	createOrgRoutes(apiGroup, mc, proxyHandler, &handler.OrganizationHandler{})

	return router
}

func TestOrgRoutes_AnonymousWriteAccess_Returns401(t *testing.T) {
	router := newOrgRoutesTestRouter(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/organizations"},
		{http.MethodPut, "/api/v1/organization/testorg"},
		{http.MethodDelete, "/api/v1/organization/testorg"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusUnauthorized, resp.Code,
				"anonymous %s %s should return 401", tc.method, tc.path)
		})
	}
}

func TestOrgRoutes_LoginUser_CreateOrg_PassesThrough(t *testing.T) {
	router := newOrgRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user POST /organizations should reach the proxy")
}

func TestMemberRoutes_AnonymousWriteAccess_Returns401(t *testing.T) {
	router := newOrgRoutesTestRouter(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/organization/testorg/members"},
		{http.MethodPut, "/api/v1/organization/testorg/members/testuser"},
		{http.MethodDelete, "/api/v1/organization/testorg/members/testuser"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusUnauthorized, resp.Code,
				"anonymous %s %s should return 401", tc.method, tc.path)
		})
	}
}

func TestMemberRoutes_LoginUser_AddMember_PassesThrough(t *testing.T) {
	router := newOrgRoutesTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/organization/testorg/members", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user POST members should reach the proxy (permission checked downstream)")
}

// =============================================================================
// GET /users (registered directly in NewRouter)
// =============================================================================

func newGetUsersTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	proxyHandler, err := handler.NewInternalServiceProxyHandler(backend.URL)
	if err != nil {
		t.Fatalf("failed to create proxy handler: %v", err)
	}

	mc := middleware.MiddlewareCollection{}
	mc.Auth.NeedLogin = middleware.MustLogin()

	router := gin.New()
	router.Use(setTestUser)

	apiGroup := router.Group("/api/v1")
	createUserRoutes(apiGroup, mc, proxyHandler, &handler.UserHandler{})

	return router
}

func TestUsersListRoute_AnonymousAccess_Returns401(t *testing.T) {
	router := newGetUsersTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code,
		"anonymous GET /users should return 401")
}

func TestUsersListRoute_LoginUser_PassesThrough(t *testing.T) {
	router := newGetUsersTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("X-Test-User", "testuser")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code,
		"logged-in user GET /users should reach the proxy (component returns limited info for non-admin)")
}
