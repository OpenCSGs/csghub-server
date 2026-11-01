//go:build !ee && !saas

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

func WithTimeBucketRateLimter(config *config.Config) RateLimiterOption {
	return func(o *rateLimiterOption) {
		o.alg = nil
		o.defaultLimit = 0
	}
}

func WithSlidingWindowRateLimter(config *config.Config) RateLimiterOption {
	return func(o *rateLimiterOption) {
		o.alg = nil
		o.defaultLimit = 0
	}
}

func WithIPCheck() RateLimiterOption {
	return func(rlo *rateLimiterOption) {
		rlo.checkIP = true
	}
}

// RateLimiter creates a middleware to limit request frequency for a specific action.
// It uses a sliding window algorithm implemented in Redis.
// If the limit is exceeded, it returns a 429 Too Many Requests error.
func RateLimiter(config *config.Config, opt ...RateLimiterOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
