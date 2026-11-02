package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
)

func TestAnonymousGuestRateLimiterCheck(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().RunScript(
		mock.Anything,
		anonymousGuestUseRateLimitScript,
		mock.Anything,
		int64(60),
	).Return(int64(11), nil).Once()

	limiter := &anonymousGuestRateLimiter{
		redisClient: redisClient,
		limit:       10,
		window:      60,
	}

	count, err := limiter.Check(context.Background(), "192.0.2.10")

	require.NoError(t, err)
	require.Equal(t, int64(11), count)
}

func TestAnonymousGuestRateLimiterCheckRejectsInvalidWindow(t *testing.T) {
	limiter := &anonymousGuestRateLimiter{limit: 10}

	count, err := limiter.Check(context.Background(), "192.0.2.10")

	require.Error(t, err)
	require.Zero(t, count)
}

func TestAnonymousGuestUseRateLimitHandlerRejectsLimitExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().RunScript(
		mock.Anything,
		anonymousGuestUseRateLimitScript,
		mock.Anything,
		int64(60),
	).Return(int64(11), nil).Once()

	called := false
	router := gin.New()
	router.POST("/warmup", anonymousGuestUseRateLimitHandler(&anonymousGuestRateLimiter{
		redisClient: redisClient,
		limit:       10,
		window:      60,
	}), func(c *gin.Context) {
		called = true
	})

	req := httptest.NewRequest("POST", "/warmup", nil)
	req.Header.Set("X-Forwarded-For", "192.0.2.10")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	require.Equal(t, 429, response.Code)
	require.Equal(t, "60", response.Header().Get("Retry-After"))
	require.False(t, called)
}
