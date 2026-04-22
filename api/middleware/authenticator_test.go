package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
	cfg.User.Host = "http://localhost"
	cfg.User.Port = 8088
	cfg.JWT.SigningKey = "test-signing-key"

	validClaims := &types.JWTClaims{
		CurrentUser: "testuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	validToken, _ := token.SignedString([]byte(cfg.JWT.SigningKey))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		handlerCalled  bool
		isJWTTest      bool
		inBlacklist    bool
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
		{
			name:           "JWT token in blacklist returns 401",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusUnauthorized,
			handlerCalled:  false,
			isJWTTest:      true,
			inBlacklist:    true,
		},
		{
			name:           "JWT token not in blacklist passes",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			handlerCalled:  true,
			isJWTTest:      true,
			inBlacklist:    false,
		},
		{
			name:           "credential runtime token passes through",
			authHeader:     "Bearer runtime-credential-token",
			expectedStatus: http.StatusOK,
			handlerCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isJWTTest {
				mockRedis := cache.NewMockRedisClient(t)
				tokenStr := strings.TrimPrefix(tt.authHeader, "Bearer ")

				mockRedis.EXPECT().SIsMember(mock.Anything, "jwt_blacklist", tokenStr).Return(tt.inBlacklist, nil)

				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("GET", "/test", nil)

				result := isValidJWTToken(c, cfg, tokenStr, mockRedis)

				if tt.expectedStatus == http.StatusOK {
					assert.True(t, result)
				} else {
					assert.False(t, result)
				}
			} else {
				router := gin.New()
				router.Use(Authenticator(cfg))

				handlerCalled := false
				path := "/test"
				if tt.name == "credential runtime token passes through" {
					path = "/api/v1/agent/credentials/runtime/github"
				}

				router.GET(path, func(c *gin.Context) {
					handlerCalled = true
					c.Status(http.StatusOK)
				})

				req := httptest.NewRequest(http.MethodGet, path, nil)
				if tt.authHeader != "" {
					req.Header.Set("Authorization", tt.authHeader)
				}
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				assert.Equal(t, tt.expectedStatus, w.Code)
				assert.Equal(t, tt.handlerCalled, handlerCalled)
			}
		})
	}
}

func TestUsesDelegatedAuth(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "runtime credential get",
			path: "/api/v1/agent/credentials/runtime/gitlab-devops",
			want: true,
		},
		{
			name: "runtime credential session revoke",
			path: "/api/v1/agent/credentials/runtime/session/revoke",
			want: true,
		},
		{
			name: "management credential route",
			path: "/api/v1/agent/credentials/gitlab-devops",
			want: false,
		},
		{
			name: "runtime prefix without child path",
			path: "/api/v1/agent/credentials/runtime",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, usesDelegatedAuth(tt.path))
		})
	}
}
