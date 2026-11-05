package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRiskRuleCacheRefreshAndFailure(t *testing.T) {
	ctx := context.Background()
	var clock atomic.Int64
	cache := &RiskRuleCache{now: func() time.Time { return time.Unix(clock.Load(), 0) }}
	var calls atomic.Int64
	var fail atomic.Bool
	load := func(context.Context) (map[string]struct{}, error) {
		calls.Add(1)
		if fail.Load() {
			return nil, errors.New("database unavailable")
		}
		return map[string]struct{}{"listed": {}}, nil
	}
	for range 1000 {
		rules, err := cache.Get(ctx, load)
		require.NoError(t, err)
		require.True(t, rules.Contains("listed"))
	}
	require.Equal(t, int64(1), calls.Load())
	clock.Store(int64(riskRuleCacheTTL / time.Second))
	fail.Store(true)
	_, err := cache.Get(ctx, load)
	require.ErrorContains(t, err, "database unavailable")
	_, err = cache.Get(ctx, load)
	require.Error(t, err) // Do not use a stale rule snapshot after refresh failure.
	require.Equal(t, int64(2), calls.Load())
	fail.Store(false)
	clock.Add(int64(riskRuleErrorTTL / time.Second))
	rules, err := cache.Get(ctx, load)
	require.NoError(t, err)
	require.True(t, rules.Contains("listed"))
	require.Equal(t, int64(3), calls.Load())
}

func TestRiskRuleCacheConcurrentColdLoad(t *testing.T) {
	var cache RiskRuleCache
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	load := func(context.Context) (map[string]struct{}, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return map[string]struct{}{"listed": {}}, nil
	}
	const requests = 32
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rules, err := cache.Get(context.Background(), load)
			if err == nil {
				if !rules.Contains("listed") {
					err = errors.New("missing cached rule")
				}
			}
			errs <- err
		}()
	}
	<-entered
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), calls.Load())
}

func TestRiskRuleCacheCallerCancellation(t *testing.T) {
	var cache RiskRuleCache
	entered := make(chan struct{})
	release := make(chan struct{})
	load := func(ctx context.Context) (map[string]struct{}, error) {
		close(entered)
		<-release
		return map[string]struct{}{"listed": {}}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := cache.Get(ctx, load)
		done <- err
	}()
	<-entered
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	close(release)
	rules, err := cache.Get(context.Background(), load)
	require.NoError(t, err)
	require.True(t, rules.Contains("listed"))
}
