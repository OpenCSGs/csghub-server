package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func captureDefaultLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })
	return &output
}

func TestBuildJWTSessionDoesNotLogInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "invalid.jwt.secret"
	logs := captureDefaultLogs(t)

	router := gin.New()
	router.Use(BuildJwtSession("signing-key"))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?jwt="+secret, nil)

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NotContains(t, logs.String(), secret)
}

func TestBlacklistedJWTDoesNotLogToken(t *testing.T) {
	const signingKey = "signing-key"
	claims := &types.JWTClaims{
		CurrentUser: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(signingKey))
	require.NoError(t, err)
	logs := captureDefaultLogs(t)

	redisClient := cache.NewMockRedisClient(t)
	redisClient.EXPECT().SIsMember(mock.Anything, "jwt_blacklist", token).Return(true, nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	cfg := &config.Config{}
	cfg.JWT.SigningKey = signingKey

	valid := isValidJWTToken(context, cfg, token, redisClient)

	require.False(t, valid)
	require.NotContains(t, logs.String(), token)
}

func TestGitlabShellMiddlewareDoesNotLogToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "invalid-gitlab-shell-jwt"
	logs := captureDefaultLogs(t)

	router := gin.New()
	router.Use(CheckGitlabShellJWTToken(&config.Config{}))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(apiSecretHeaderName, secret)

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NotContains(t, logs.String(), secret)
}

func TestGitHTTPMiddlewareDoesNotLogBasicHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "%%%invalid-basic-secret%%%"
	logs := captureDefaultLogs(t)

	router := gin.New()
	router.Use(GetCurrentUserFromHeader())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(types.HeaderAuthorization, "Basic "+secret)

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, logs.String(), secret)
}

func TestAccessLogDoesNotExposePathOrOAuthCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []string{"/jwt/:token", "/token/:token_value", "/token/:token_value/quotas", "/api/v1/fedap/oauth/callback", "/api/v1/callback/casdoor"} {
		for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
			t.Run(route+http.StatusText(status), func(t *testing.T) {
				logs := captureDefaultLogs(t)
				router := gin.New()
				router.Use(Log())
				router.GET(route, func(c *gin.Context) { c.AbortWithStatus(status) })
				path := strings.ReplaceAll(strings.ReplaceAll(route, ":token_value", "path-secret"), ":token", "path-secret")
				request := httptest.NewRequest(http.MethodGet, path+"?code=code-secret&state=state-secret&plain=visible", nil)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, status, response.Code)
				require.NotContains(t, logs.String(), "path-secret")
				if strings.HasSuffix(route, "/oauth/callback") || strings.HasSuffix(route, "/callback/casdoor") {
					require.NotContains(t, logs.String(), "code-secret")
					require.NotContains(t, logs.String(), "state-secret")
				}
				require.Contains(t, logs.String(), "visible")
				require.Contains(t, logs.String(), route)
			})
		}
	}
}
