package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

// limiterAlgorithm defines the interface for a rate limiting algorithm.
type limiterAlgorithm interface {
	Check(ctx context.Context, action, userID string, limit int64) (int64, error)
}

type rateLimitConfig struct {
	Enable bool
	Limit  int64
	Window int64
}

type rateLimiterOption struct {
	alg             limiterAlgorithm
	checkIP         bool
	onLimitExceeded func(*gin.Context)
	rateLimitConfig *rateLimitConfig
}

type RateLimiterOption func(*rateLimiterOption)

// WithOnLimitExceeded sets a custom handler to be called when the rate limit is exceeded.
// When set, the middleware aborts the request chain instead of delegating to captcha.
func WithOnLimitExceeded(handler func(*gin.Context)) RateLimiterOption {
	return func(o *rateLimiterOption) {
		o.onLimitExceeded = handler
	}
}

// WithRateLimitConfig overrides the global APIRateLimiter config with explicit values.
func WithRateLimitConfig(enable bool, limit, window int64) RateLimiterOption {
	return func(o *rateLimiterOption) {
		o.rateLimitConfig = &rateLimitConfig{
			Enable: enable,
			Limit:  limit,
			Window: window,
		}
	}
}

// WithLimiterAlgorithm allows injecting a custom rate limiting algorithm.
func WithLimiterAlgorithm(alg limiterAlgorithm) RateLimiterOption {
	return func(o *rateLimiterOption) {
		o.alg = alg
	}
}

