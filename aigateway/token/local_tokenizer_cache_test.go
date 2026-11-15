package token

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenizerCacheColdReadsDoNotWaitAndFailureRetries(t *testing.T) {
	now := time.Now()
	started, release := make(chan struct{}, 2), make(chan struct{})
	calls := 0
	cache := tokenizerCache{now: func() time.Time { return now }, retryDelay: time.Minute, timeout: time.Second,
		load: func(ctx context.Context) Tokenizer {
			calls++
			started <- struct{}{}
			<-release
			if calls == 1 {
				return nil
			}
			return &DumyTokenizer{}
		}}
	require.Nil(t, cache.get())
	<-started
	for i := 0; i < 20; i++ {
		require.Nil(t, cache.get())
	}
	require.Equal(t, 1, calls)
	close(release)
	require.Nil(t, cache.wait(context.Background()))
	require.Nil(t, cache.get())
	require.Equal(t, 1, calls)
	now = now.Add(time.Minute)
	require.Nil(t, cache.get())
	<-started
	require.NotNil(t, cache.wait(context.Background()))
	require.Equal(t, 2, calls)
}

func TestTokenizerCacheWaitHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	cache := tokenizerCache{now: time.Now, retryDelay: time.Minute, timeout: time.Second,
		load: func(ctx context.Context) Tokenizer { close(started); <-ctx.Done(); return nil }}
	require.Nil(t, cache.get())
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Nil(t, cache.wait(ctx))
}

func TestVocabularyFetchHonorsCancelledContext(t *testing.T) {
	t.Setenv("TIKTOKEN_CACHE_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, ensureCl100kCache(ctx), context.Canceled)
}
