//go:build !ee && !saas

package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
)

type slidingWindowLimiter struct {
	redisClient cache.RedisClient
	window      int64
	limit       int64
}

func (s *slidingWindowLimiter) Check(ctx context.Context, action, userID string, limit int64) (int64, error) {
	if s.redisClient == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	// Defensive check: if window is not configured or invalid, bypass limiting
	if s.window <= 0 {
		return limit, nil
	}

	key := fmt.Sprintf("rate_limit:sw:%s:%s", action, userID)
	now := time.Now().UnixNano() / int64(time.Millisecond)
	windowMs := s.window * 1000
	member := uuid.NewString()

	const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local current_requests = redis.call('ZCARD', key)

if current_requests < limit then
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, math.ceil(window / 1000) + 10)
    return limit - current_requests - 1
else
    return -1
end
`

	res, err := s.redisClient.RunScript(ctx, slidingWindowScript, []string{key}, now, windowMs, limit, member)
	if err != nil {
		return 0, fmt.Errorf("running sliding window script: %w", err)
	}

	var remaining int64 = -1
	switch v := res.(type) {
	case int64:
		remaining = v
	case int:
		remaining = int64(v)
	}

	return remaining, nil
}

type tokenBucketLimiter struct {
	redisClient cache.RedisClient
	window      int64
	limit       int64
}

func (t *tokenBucketLimiter) Check(ctx context.Context, action, userID string, limit int64) (int64, error) {
	if t.redisClient == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	// Defensive check: if window is not configured or invalid, bypass limiting
	if t.window <= 0 {
		return limit, nil
	}

	key := fmt.Sprintf("rate_limit:tb:%s:%s", action, userID)
	now := time.Now().Unix()

	const tokenBucketScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = 1

local refill_rate = limit / window
local ttl = window

local data = redis.call('HMGET', key, 'tokens', 'last_refilled_at')
local tokens = tonumber(data[1])
local last_refilled_at = tonumber(data[2])

if not tokens then
    tokens = limit
    last_refilled_at = now
else
    local elapsed = now - last_refilled_at
    if elapsed > 0 then
        tokens = math.min(limit, tokens + elapsed * refill_rate)
        last_refilled_at = now
    elseif elapsed < 0 then
        -- Handle potential system clock drift/reset
        last_refilled_at = now
    end
end

if tokens >= cost then
    tokens = tokens - cost
    redis.call('HMSET', key, 'tokens', tokens, 'last_refilled_at', last_refilled_at)
    redis.call('EXPIRE', key, ttl)
    return math.floor(tokens)
else
    redis.call('HMSET', key, 'tokens', tokens, 'last_refilled_at', last_refilled_at)
    redis.call('EXPIRE', key, ttl)
    return -1
end
`

	res, err := t.redisClient.RunScript(ctx, tokenBucketScript, []string{key}, limit, t.window, now)
	if err != nil {
		return 0, fmt.Errorf("running token bucket script: %w", err)
	}

	var remaining int64 = -1
	switch v := res.(type) {
	case int64:
		remaining = v
	case int:
		remaining = int64(v)
	}

	return remaining, nil
}

func WithTimeBucketRateLimter(config *config.Config) RateLimiterOption {
	redisClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		slog.Error("failed to initialize redis client in time bucket rate limiter", slog.Any("error", err))
	}
	return func(o *rateLimiterOption) {
		o.alg = &tokenBucketLimiter{
			redisClient: redisClient,
		}
	}
}

func WithSlidingWindowRateLimter(config *config.Config) RateLimiterOption {
	redisClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		slog.Error("failed to initialize redis client in sliding window rate limiter", slog.Any("error", err))
	}
	return func(o *rateLimiterOption) {
		o.alg = &slidingWindowLimiter{
			redisClient: redisClient,
		}
	}
}

func WithIPCheck() RateLimiterOption {
	return func(rlo *rateLimiterOption) {
		rlo.checkIP = true
	}
}

// RateLimiter creates a middleware to limit request frequency for a specific action.
// It uses a sliding window algorithm implemented in Redis by default.
// If the limit is exceeded, it returns a 429 Too Many Requests error.
func RateLimiter(config *config.Config, opt ...RateLimiterOption) gin.HandlerFunc {
	o := &rateLimiterOption{
		rateLimitConfig: &rateLimitConfig{
			Enable: config.APIRateLimiter.Enable,
			Limit:  config.APIRateLimiter.Limit,
			Window: config.APIRateLimiter.Window,
		},
	}
	for _, option := range opt {
		option(o)
	}

	if o.rateLimitConfig != nil && !o.rateLimitConfig.Enable {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Configure algorithm metrics
	if sw, ok := o.alg.(*slidingWindowLimiter); ok && sw != nil {
		sw.window = o.rateLimitConfig.Window
		sw.limit = o.rateLimitConfig.Limit
	} else if tb, ok := o.alg.(*tokenBucketLimiter); ok && tb != nil {
		tb.window = o.rateLimitConfig.Window
		tb.limit = o.rateLimitConfig.Limit
	}

	// Fallback to sliding window if alg is nil
	if o.alg == nil {
		redisClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
			Addr:     config.Redis.Endpoint,
			Username: config.Redis.User,
			Password: config.Redis.Password,
		})
		if err != nil {
			slog.Error("failed to initialize fallback redis client in rate limiter", slog.Any("error", err))
		}
		o.alg = &slidingWindowLimiter{
			redisClient: redisClient,
			window:      o.rateLimitConfig.Window,
			limit:       o.rateLimitConfig.Limit,
		}
	}

	return func(c *gin.Context) {
		var identifier string
		if o.checkIP {
			identifier = httpbase.GetIPAddress(c)
			if identifier == "" {
				identifier = c.ClientIP()
			}
		} else {
			identifier = httpbase.GetCurrentUserUUID(c)
			if identifier == "" {
				identifier = httpbase.GetCurrentUser(c)
			}
			if identifier == "" {
				identifier = httpbase.GetIPAddress(c)
				if identifier == "" {
					identifier = c.ClientIP()
				}
			}
		}

		action := c.FullPath()
		if action == "" {
			action = c.Request.URL.Path
		}

		remaining, err := o.alg.Check(c.Request.Context(), action, identifier, o.rateLimitConfig.Limit)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "rate limit check failed, allowing request", slog.String("action", action), slog.String("user", identifier), slog.Any("error", err))
			c.Next()
			return
		}

		// RFC 6585 compliant rate-limiting HTTP headers
		c.Writer.Header().Set("X-RateLimit-Limit", strconv.FormatInt(o.rateLimitConfig.Limit, 10))
		if remaining >= 0 {
			c.Writer.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		} else {
			c.Writer.Header().Set("X-RateLimit-Remaining", "0")
			c.Writer.Header().Set("Retry-After", strconv.FormatInt(o.rateLimitConfig.Window, 10))
		}

		if remaining < 0 {
			if o.onLimitExceeded != nil {
				o.onLimitExceeded(c)
			} else {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded. Please try again later.",
				})
			}
			return
		}

		c.Next()
	}
}
