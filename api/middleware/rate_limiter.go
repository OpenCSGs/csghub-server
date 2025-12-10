package middleware

import "context"

// limiterAlgorithm defines the interface for a rate limiting algorithm.
type limiterAlgorithm interface {
	Check(ctx context.Context, action, userID string) (int64, error)
}

type rateLimiterOption struct {
	alg          limiterAlgorithm
	defaultLimit int64
	checkIP      bool
}

type RateLimiterOption func(*rateLimiterOption)
