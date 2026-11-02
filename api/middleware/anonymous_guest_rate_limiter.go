package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
)

const (
	anonymousGuestUseRateLimit  = int64(10)
	anonymousGuestUseRateWindow = int64(60)
)

const anonymousGuestUseRateLimitScript = `
local current = redis.call('INCR', KEYS[1])
if tonumber(current) == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`

// AnonymousGuestUseRateLimit limits anonymous logical-agent warmup requests by
// client IP for one fixed time window.
func AnonymousGuestUseRateLimit(config *config.Config) gin.HandlerFunc {
	redisClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		slog.Error("failed to initialize anonymous guest rate limiter", "error", err)
		return func(c *gin.Context) {
			c.Next()
		}
	}

	limiter := &anonymousGuestRateLimiter{
		redisClient: redisClient,
		limit:       anonymousGuestUseRateLimit,
		window:      anonymousGuestUseRateWindow,
	}
	return anonymousGuestUseRateLimitHandler(limiter)
}

func anonymousGuestUseRateLimitHandler(limiter *anonymousGuestRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := limiter.Check(c.Request.Context(), c.ClientIP())
		if err != nil {
			// Keep this short-term defense fail-open, matching the existing API
			// limiter behavior when Redis is unavailable.
			slog.ErrorContext(c.Request.Context(), "failed to execute anonymous guest rate limiter", "error", err)
			c.Next()
			return
		}

		if count > limiter.limit {
			c.Header("Retry-After", fmt.Sprintf("%d", limiter.window))
			httpbase.TooManyReqError(c, errors.New("anonymous guest warmup rate limit exceeded"))
			c.Abort()
			return
		}

		c.Next()
	}
}

type anonymousGuestRateLimiter struct {
	redisClient cache.RedisClient
	limit       int64
	window      int64
}

func (l *anonymousGuestRateLimiter) Check(ctx context.Context, clientIP string) (int64, error) {
	if l.window <= 0 {
		return 0, errors.New("anonymous guest rate limiter window must be positive")
	}
	if l.limit <= 0 {
		return 0, errors.New("anonymous guest rate limiter limit must be positive")
	}

	bucket := time.Now().Unix() / l.window
	ipHash := sha256.Sum256([]byte(clientIP))
	key := fmt.Sprintf("rate_limit:anonymous_guest_use:%s:%d", hex.EncodeToString(ipHash[:]), bucket)
	result, err := l.redisClient.RunScript(ctx, anonymousGuestUseRateLimitScript, []string{key}, l.window)
	if err != nil {
		return 0, fmt.Errorf("execute anonymous guest rate limiter: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected anonymous guest rate limiter result type %T", result)
	}
	return count, nil
}
