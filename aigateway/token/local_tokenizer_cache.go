package token

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

const tokenizerLoadTimeout = 10 * time.Second

// tokenizerCache owns one in-flight load and caches successes, not failures.
// Request paths use get; existing billing callers may use bounded wait.
type tokenizerCache struct {
	mu                  sync.Mutex
	value               Tokenizer
	loading             chan struct{}
	retryAt             time.Time
	now                 func() time.Time
	timeout, retryDelay time.Duration
	load                func(context.Context) Tokenizer
}

func (c *tokenizerCache) get() Tokenizer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.value != nil {
		return c.value
	}
	if c.loading != nil || c.now().Before(c.retryAt) {
		return nil
	}
	done := make(chan struct{})
	c.loading = done
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		value := c.load(ctx)
		c.mu.Lock()
		c.value = value
		if value == nil {
			c.retryAt = c.now().Add(c.retryDelay)
		}
		c.loading = nil
		close(done)
		c.mu.Unlock()
	}()
	return nil
}

func (c *tokenizerCache) wait(ctx context.Context) Tokenizer {
	if value := c.get(); value != nil {
		return value
	}
	c.mu.Lock()
	done := c.loading
	c.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return nil
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

var sharedLocalTokenizer = tokenizerCache{
	now: time.Now, timeout: tokenizerLoadTimeout, retryDelay: 30 * time.Second,
	load: func(ctx context.Context) Tokenizer {
		// Prefill the SDK's existing cache with bounded HTTP before GetEncoding.
		// No global SDK loader or http.DefaultClient is mutated.
		if err := ensureCl100kCache(ctx); err != nil {
			slog.Warn("local tokenizer vocabulary unavailable", "error", err)
			return nil
		}
		tk, err := tiktoken.GetEncoding(tiktoken.MODEL_CL100K_BASE)
		if err != nil {
			slog.Warn("local tokenizer initialization failed", "error", err)
			return nil
		}
		return &tiktokenTokenizerImpl{enc: &tiktokenPieceAdapter{tk: tk}}
	},
}
