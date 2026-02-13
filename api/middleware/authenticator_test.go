package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
)

func TestRestrictMultiSyncTokenToRead(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authType       httpbase.AuthType
		method         string
		expectedStatus int
		shouldAbort    bool
	}{
		{
			name:           "MultiSyncToken with GET should pass",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "GET",
			expectedStatus: http.StatusOK,
			shouldAbort:    false,
		},
		{
			name:           "MultiSyncToken with HEAD should pass",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "HEAD",
			expectedStatus: http.StatusOK,
			shouldAbort:    false,
		},
		{
			name:           "MultiSyncToken with POST should be forbidden",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "POST",
			expectedStatus: http.StatusForbidden,
			shouldAbort:    true,
		},
		{
			name:           "MultiSyncToken with PUT should be forbidden",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "PUT",
			expectedStatus: http.StatusForbidden,
			shouldAbort:    true,
		},
		{
			name:           "MultiSyncToken with DELETE should be forbidden",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "DELETE",
			expectedStatus: http.StatusForbidden,
			shouldAbort:    true,
		},
		{
			name:           "MultiSyncToken with PATCH should be forbidden",
			authType:       httpbase.AuthTypeMultiSyncToken,
			method:         "PATCH",
			expectedStatus: http.StatusForbidden,
			shouldAbort:    true,
		},
		{
			name:           "JWT with POST should pass",
			authType:       httpbase.AuthTypeJwt,
			method:         "POST",
			expectedStatus: http.StatusOK,
			shouldAbort:    false,
		},
		{
			name:           "AccessToken with POST should pass",
			authType:       httpbase.AuthTypeAccessToken,
			method:         "POST",
			expectedStatus: http.StatusOK,
			shouldAbort:    false,
		},
		{
			name:           "ApiKey with DELETE should pass",
			authType:       httpbase.AuthTypeApiKey,
			method:         "DELETE",
			expectedStatus: http.StatusOK,
			shouldAbort:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				httpbase.SetAuthType(c, tt.authType)
				httpbase.SetCurrentUser(c, "testuser")
				c.Next()
			})
			router.Use(RestrictMultiSyncTokenToRead())

			handlerCalled := false
			router.Handle(tt.method, "/test", func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.shouldAbort {
				assert.Equal(t, tt.expectedStatus, w.Code, "Expected status code to match")
				assert.False(t, handlerCalled, "Handler should not be called when request is aborted")
			} else {
				assert.Equal(t, tt.expectedStatus, w.Code, "Expected status code to match")
			}
		})
	}
}

func TestRestrictMultiSyncTokenToRead_CloneOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		httpbase.SetAuthType(c, httpbase.AuthTypeMultiSyncToken)
		httpbase.SetCurrentUser(c, "testuser")
		c.Next()
	})
	router.Use(RestrictMultiSyncTokenToRead())

	router.GET("/api/v1/models/:namespace/:name/resolve/:ref/*filepath", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/models/user/model-name/resolve/main/file.txt", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Clone operation (GET) should be allowed for MultiSyncToken")
}

func TestAuthenticator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	apiToken := "test-api-token-for-ut"
	cfg := &config.Config{}
	cfg.APIToken = apiToken
	// User.Host/Port used when verifying access token; use default to avoid scheme error in invalid-token test
	cfg.User.Host = "http://localhost"
	cfg.User.Port = 8088

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		handlerCalled  bool
	}{
		{
			name:           "no Authorization header passes through",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			handlerCalled:  true,
		},
		{
			name:           "invalid scheme returns 401",
			authHeader:     "Digest xxx",
			expectedStatus: http.StatusUnauthorized,
			handlerCalled:  false,
		},
		{
			name:           "Bearer with valid API token passes",
			authHeader:     "Bearer " + apiToken,
			expectedStatus: http.StatusOK,
			handlerCalled:  true,
		},
		{
			name:           "Bearer with invalid token returns 401",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			handlerCalled:  false,
		},
		{
			name:           "Basic with invalid token returns 401",
			authHeader:     "Basic invalid-base64-or-bad-credentials",
			expectedStatus: http.StatusUnauthorized,
			handlerCalled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Authenticator(cfg))

			handlerCalled := false
			router.GET("/test", func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.handlerCalled, handlerCalled)
		})
	}
}
