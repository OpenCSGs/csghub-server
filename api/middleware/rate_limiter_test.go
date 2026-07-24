package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
)

type mockLimiterAlgorithm struct {
	remaining int64
	err       error
}

func (m *mockLimiterAlgorithm) Check(ctx context.Context, action, userID string, limit int64) (int64, error) {
	return m.remaining, m.err
}

func TestRateLimiter_Enabled_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		APIRateLimiter: struct {
			Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
			Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
			Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
		}{
			Enable: true,
			Limit:  10,
			Window: 60,
		},
	}

	mockAlg := &mockLimiterAlgorithm{remaining: 9, err: nil}

	r.Use(RateLimiter(cfg, WithLimiterAlgorithm(mockAlg)))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Assert rate-limit headers
	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "9", w.Header().Get("X-RateLimit-Remaining"))
	assert.Empty(t, w.Header().Get("Retry-After"))
}

func TestRateLimiter_Enabled_Exceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		APIRateLimiter: struct {
			Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
			Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
			Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
		}{
			Enable: true,
			Limit:  10,
			Window: 60,
		},
	}

	mockAlg := &mockLimiterAlgorithm{remaining: -1, err: nil}

	r.Use(RateLimiter(cfg, WithLimiterAlgorithm(mockAlg)))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Assert rate-limit headers for exceeded request
	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestRateLimiter_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		APIRateLimiter: struct {
			Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
			Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
			Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
		}{
			Enable: false,
			Limit:  10,
			Window: 60,
		},
	}

	mockAlg := &mockLimiterAlgorithm{remaining: -1, err: nil}

	r.Use(RateLimiter(cfg, WithLimiterAlgorithm(mockAlg)))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))
}

func TestRateLimiter_FailSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		APIRateLimiter: struct {
			Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
			Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
			Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
		}{
			Enable: true,
			Limit:  10,
			Window: 60,
		},
	}

	mockAlg := &mockLimiterAlgorithm{remaining: 0, err: errors.New("redis failure")}

	r.Use(RateLimiter(cfg, WithLimiterAlgorithm(mockAlg)))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Fail-safe should not inject limiting headers since algorithm didn't run successfully
	assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))
}

func TestRateLimiter_IPCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		APIRateLimiter: struct {
			Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
			Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
			Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
		}{
			Enable: true,
			Limit:  10,
			Window: 60,
		},
	}

	var capturedUser string
	mockAlg := gin.HandlerFunc(func(c *gin.Context) {
		c.Set(httpbase.IPctxVar, "1.2.3.4")
		c.Next()
	})

	r.Use(mockAlg)
	r.Use(func(c *gin.Context) {
		// Custom rate limiter wrapping mockAlg
		limiterOpt := WithLimiterAlgorithm(ginLimiterAlgFunc(func(ctx context.Context, action, userID string, limit int64) (int64, error) {
			capturedUser = userID
			return 9, nil
		}))
		RateLimiter(cfg, limiterOpt, WithIPCheck())(c)
	})

	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "1.2.3.4", capturedUser)
}

type ginLimiterAlgFunc func(ctx context.Context, action, userID string, limit int64) (int64, error)

func (f ginLimiterAlgFunc) Check(ctx context.Context, action, userID string, limit int64) (int64, error) {
	return f(ctx, action, userID, limit)
}

func TestSlidingWindowLimiter_Check(t *testing.T) {
	mockRedis := mock_cache.NewMockRedisClient(t)
	limiter := &slidingWindowLimiter{
		redisClient: mockRedis,
		window:      60,
		limit:       10,
	}

	ctx := context.Background()

	// 1. Success case: returns remaining
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:sw:test_action:test_user"}, mock.Anything, mock.Anything, int64(10), mock.Anything).
		Return(int64(9), nil).
		Once()

	rem, err := limiter.Check(ctx, "test_action", "test_user", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(9), rem)

	// 2. Exceeded case: returns -1
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:sw:test_action:test_user"}, mock.Anything, mock.Anything, int64(10), mock.Anything).
		Return(int64(-1), nil).
		Once()

	rem, err = limiter.Check(ctx, "test_action", "test_user", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), rem)

	// 3. Error case
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:sw:test_action:test_user"}, mock.Anything, mock.Anything, int64(10), mock.Anything).
		Return(nil, errors.New("redis err")).
		Once()

	_, err = limiter.Check(ctx, "test_action", "test_user", 10)
	assert.Error(t, err)
}

func TestTokenBucketLimiter_Check(t *testing.T) {
	mockRedis := mock_cache.NewMockRedisClient(t)
	limiter := &tokenBucketLimiter{
		redisClient: mockRedis,
		window:      60,
		limit:       10,
	}

	ctx := context.Background()

	// 1. Success case: returns remaining
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:tb:test_action:test_user"}, int64(10), int64(60), mock.Anything).
		Return(int64(9), nil).
		Once()

	rem, err := limiter.Check(ctx, "test_action", "test_user", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(9), rem)

	// 2. Exceeded case: returns -1
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:tb:test_action:test_user"}, int64(10), int64(60), mock.Anything).
		Return(int64(-1), nil).
		Once()

	rem, err = limiter.Check(ctx, "test_action", "test_user", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), rem)

	// 3. Error case
	mockRedis.EXPECT().
		RunScript(ctx, mock.Anything, []string{"rate_limit:tb:test_action:test_user"}, int64(10), int64(60), mock.Anything).
		Return(nil, errors.New("redis err")).
		Once()

	_, err = limiter.Check(ctx, "test_action", "test_user", 10)
	assert.Error(t, err)
}
