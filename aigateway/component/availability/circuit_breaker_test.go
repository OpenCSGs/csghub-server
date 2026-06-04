package availability

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/database"
)

type fakeCircuitStore struct {
	mu     sync.Mutex
	states map[string]*database.AIGatewayUpstreamCircuitState
}

func newFakeCircuitStore() *fakeCircuitStore {
	return &fakeCircuitStore{states: map[string]*database.AIGatewayUpstreamCircuitState{}}
}

func (s *fakeCircuitStore) key(upstreamID int64) string {
	return strconv.FormatInt(upstreamID, 10)
}

func (s *fakeCircuitStore) Create(ctx context.Context, state *database.AIGatewayUpstreamCircuitState) error {
	return s.Upsert(ctx, state)
}

func (s *fakeCircuitStore) Update(ctx context.Context, state *database.AIGatewayUpstreamCircuitState) error {
	return s.Upsert(ctx, state)
}

func (s *fakeCircuitStore) Upsert(_ context.Context, state *database.AIGatewayUpstreamCircuitState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := *state
	s.states[s.key(state.UpstreamID)] = &copied
	return nil
}

func (s *fakeCircuitStore) GetByUpstreamID(_ context.Context, upstreamID int64) (*database.AIGatewayUpstreamCircuitState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[s.key(upstreamID)]
	if !ok {
		return nil, errors.New("not found")
	}
	copied := *state
	return &copied, nil
}

func (s *fakeCircuitStore) GetAllOpen(_ context.Context) ([]database.AIGatewayUpstreamCircuitState, error) {
	return nil, nil
}
func (s *fakeCircuitStore) GetAllClosed(_ context.Context) ([]database.AIGatewayUpstreamCircuitState, error) {
	return nil, nil
}

func (s *fakeCircuitStore) DeleteByUpstreamID(_ context.Context, upstreamID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, s.key(upstreamID))
	return nil
}

func TestCircuitBreaker_IsAvailable_HalfOpenLimitReached(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	store := newFakeCircuitStore()
	cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true, HalfOpenMaxRequests: 1}, store, redisClient)

	redisClient.EXPECT().
		HGetAll(context.Background(), "aigateway:availability:circuit:1").
		Return(map[string]string{
			"circuit_state":     "half_open",
			"failure_count":     "0",
			"success_count":     "0",
			"last_state_change": fmt.Sprintf("%d", time.Now().Unix()),
		}, nil).
		Once()
	redisClient.EXPECT().
		RunScript(context.Background(), incrementHalfOpenRequestsScript, []string{"aigateway:availability:circuit:half-open:1"}, 1, int((10*time.Second).Seconds())).
		Return([]any{int64(0), int64(1)}, nil).
		Once()

	available, err := cb.IsAvailable(context.Background(), int64(1))
	require.NoError(t, err)
	require.False(t, available)
}

func TestCircuitBreaker_RecordFailure_OpensCircuit(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	store := newFakeCircuitStore()
	cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true, FailureThreshold: 3, OpenDuration: 30 * time.Second}, store, redisClient)

	now := time.Now().Unix()
	redisClient.EXPECT().
		RunScript(context.Background(), recordFailureScript, []string{
			"aigateway:availability:circuit:1",
			"aigateway:availability:circuit:half-open:1",
		}, 3, 30, mock.Anything, 10).
		Return([]any{"open", int64(0), int64(0), now, now + 30}, nil).
		Once()
	redisClient.EXPECT().
		HMSet(context.Background(), "aigateway:availability:circuit:1", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().Expire(context.Background(), "aigateway:availability:circuit:1", 10*time.Second).Return(nil).Once()
	invalidateDone := make(chan struct{}, 1)
	redisClient.EXPECT().HDel(mock.Anything, modelCacheKey, "test-model(OpenAI)").
		Run(func(ctx context.Context, key string, fields ...string) {
			invalidateDone <- struct{}{}
		}).Return(nil).Once()

	err := cb.RecordFailure(context.Background(), int64(1), "test-model(OpenAI)", errors.New("boom"))
	require.NoError(t, err)
	select {
	case <-invalidateDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async cache invalidation")
	}

	state, err := store.GetByUpstreamID(context.Background(), int64(1))
	require.NoError(t, err)
	require.Equal(t, string(types.CircuitStateOpen), state.CircuitState)
}

// TestCircuitBreaker_RecordSuccess_FallbackPath tests RecordSuccess when
// stateCache is disabled (nil redisClient). This exercises the local cache /
// DB fallback path and state transition logic at circuit_breaker.go:189-206.
func TestCircuitBreaker_RecordSuccess_FallbackPath(t *testing.T) {
	// Helper: pre-populate the circuit store with a given state.
	seedStore := func(store *fakeCircuitStore, upstreamID int64, state types.CircuitState) {
		now := time.Now()
		_ = store.Upsert(context.Background(), &database.AIGatewayUpstreamCircuitState{
			UpstreamID:      upstreamID,
			CircuitState:    string(state),
			FailureCount:    0,
			SuccessCount:    0,
			LastStateChange: now,
			NextRetryAt:     nil,
		})
	}

	t.Run("half_open transitions to closed after success", func(t *testing.T) {
		store := newFakeCircuitStore()
		seedStore(store, 1, types.CircuitStateHalfOpen)
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil)

		err := cb.RecordSuccess(context.Background(), int64(1))
		require.NoError(t, err)

		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), dbState.CircuitState)
		require.Equal(t, 0, dbState.SuccessCount)
		require.Equal(t, 0, dbState.FailureCount)
	})

	t.Run("closed increments success_count without changing state", func(t *testing.T) {
		store := newFakeCircuitStore()
		seedStore(store, 1, types.CircuitStateClosed)
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil)

		err := cb.RecordSuccess(context.Background(), int64(1))
		require.NoError(t, err)

		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), dbState.CircuitState)
		require.Equal(t, 1, dbState.SuccessCount)
		require.Equal(t, 0, dbState.FailureCount)
	})

	t.Run("getCircuitState db error creates default closed and records success", func(t *testing.T) {
		// No state in store → GetByUpstreamID returns "not found" →
		// getCircuitState creates newDefaultCircuitStatus (closed).
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil)

		err := cb.RecordSuccess(context.Background(), int64(1))
		require.NoError(t, err)

		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), dbState.CircuitState)
		require.Equal(t, 1, dbState.SuccessCount)
	})
}

// TestCircuitBreaker_RecordSuccess_LocalCache tests the local cache hit/miss
// behavior when stateCache is disabled, verifying that:
//  1. After persistCircuitState sets localCache, the next getCircuitState
//     returns the cached value without a DB call.
//  2. Invalidating localCache forces a DB re-read.
//  3. The cached state reflects the latest state after a transition.
func TestCircuitBreaker_RecordSuccess_LocalCache(t *testing.T) {
	const upstreamID = int64(42)

	t.Run("local cache hit avoids db read", func(t *testing.T) {
		store := newFakeCircuitStore()
		// Pre-populate DB with open state. A subsequent GetByUpstreamID will
		// fail because the RecordSuccess fallback path writes the updated
		// state — but we want to verify the cache hit path by making the
		// DB return something different from what we expect.
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil).(*circuitBreakerImpl)

		// Step 1: persist half_open to DB + localCache so localCache has half_open.
		now := time.Now()
		halfOpen := &types.ProviderCircuitStatus{
			UpstreamID:      upstreamID,
			CircuitState:    types.CircuitStateHalfOpen,
			FailureCount:    0,
			SuccessCount:    0,
			LastStateChange: now,
		}
		require.NoError(t, cb.persistCircuitState(context.Background(), halfOpen))
		// Verify DB also has half_open.
		dbState, err := store.GetByUpstreamID(context.Background(), upstreamID)
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateHalfOpen), dbState.CircuitState)

		// Step 2: manually overwrite DB to a different state (closed).
		// This simulates another instance making a change.
		require.NoError(t, store.Upsert(context.Background(), &database.AIGatewayUpstreamCircuitState{
			UpstreamID:      upstreamID,
			CircuitState:    string(types.CircuitStateClosed),
			FailureCount:    99, // should not be read if cache hits
			SuccessCount:    99,
			LastStateChange: now,
		}))

		// Step 3: call RecordSuccess. With localCache valid, getCircuitState
		// should return the cached half_open, NOT the DB's closed with 99 counts.
		require.NoError(t, cb.RecordSuccess(context.Background(), upstreamID))

		// After RecordSuccess, the half_open should transition to closed.
		// The DB should now have closed with SuccessCount=0, FailureCount=0
		// (from the transition), NOT the 99 values we wrote.
		finalDB, err := store.GetByUpstreamID(context.Background(), upstreamID)
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), finalDB.CircuitState)
		require.Equal(t, 0, finalDB.SuccessCount, "should be 0 after half_open→closed transition")
		require.Equal(t, 0, finalDB.FailureCount, "should be 0 after transition")
	})

	t.Run("invalidating local cache forces db re-read", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil).(*circuitBreakerImpl)

		// Populate localCache with open state.
		now := time.Now()
		openState := &types.ProviderCircuitStatus{
			UpstreamID:   upstreamID,
			CircuitState: types.CircuitStateOpen,
			NextRetryAt:  &now,
		}
		require.NoError(t, cb.persistCircuitState(context.Background(), openState))

		// Overwrite DB with closed state (simulating another instance).
		require.NoError(t, store.Upsert(context.Background(), &database.AIGatewayUpstreamCircuitState{
			UpstreamID:   upstreamID,
			CircuitState: string(types.CircuitStateClosed),
		}))

		// Invalidate localCache — next getCircuitState must hit DB.
		cb.invalidateLocalCache(cb.localCacheKey(upstreamID))

		// Now call RecordSuccess. getCircuitState will miss local cache,
		// hit DB, and get closed state. SuccessCount should increment to 1.
		require.NoError(t, cb.RecordSuccess(context.Background(), upstreamID))

		finalDB, err := store.GetByUpstreamID(context.Background(), upstreamID)
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), finalDB.CircuitState)
		require.Equal(t, 1, finalDB.SuccessCount)
	})

	t.Run("repeated record success accumulates via cache", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil).(*circuitBreakerImpl)

		// First call: no localCache, reads from DB (not found → default closed).
		require.NoError(t, cb.RecordSuccess(context.Background(), upstreamID))
		db1, _ := store.GetByUpstreamID(context.Background(), upstreamID)
		require.Equal(t, 1, db1.SuccessCount)

		// Second call: localCache hit → returns state with SuccessCount=1 from cache.
		require.NoError(t, cb.RecordSuccess(context.Background(), upstreamID))
		db2, _ := store.GetByUpstreamID(context.Background(), upstreamID)
		require.Equal(t, 2, db2.SuccessCount)

		// Third call: cache hit again.
		require.NoError(t, cb.RecordSuccess(context.Background(), upstreamID))
		db3, _ := store.GetByUpstreamID(context.Background(), upstreamID)
		require.Equal(t, 3, db3.SuccessCount)
	})
}

// TestCircuitBreaker_RecordSuccess_RedisPath tests RecordSuccess when
// stateCache is enabled (non-nil redisClient). This covers the Redis
// script path and verifies the returned status is persisted correctly.
func TestCircuitBreaker_RecordSuccess_RedisPath(t *testing.T) {
	t.Run("half_open to closed via redis", func(t *testing.T) {
		redisClient := mockcache.NewMockRedisClient(t)
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, redisClient)

		nowUnix := time.Now().Unix()

		// Redis RecordSuccess script: half_open → closed
		redisClient.EXPECT().
			RunScript(context.Background(), recordSuccessScript, []string{
				"aigateway:availability:circuit:1",
				"aigateway:availability:circuit:half-open:1",
			}, mock.Anything, mock.Anything).
			Return([]any{"closed", int64(0), int64(0), nowUnix, int64(0)}, nil).
			Once()

		// persistCircuitState → SetCircuitState → HMSet + HDel + Expire
		redisClient.EXPECT().
			HMSet(context.Background(), "aigateway:availability:circuit:1", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		redisClient.EXPECT().
			HDel(context.Background(), "aigateway:availability:circuit:1", "next_retry_at").
			Return(nil).Once()
		redisClient.EXPECT().
			Expire(context.Background(), "aigateway:availability:circuit:1", 10*time.Second).
			Return(nil).Once()

		err := cb.RecordSuccess(context.Background(), int64(1))
		require.NoError(t, err)

		// Verify state persisted to DB.
		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), dbState.CircuitState)
	})

	t.Run("redis error falls back to db path", func(t *testing.T) {
		redisClient := mockcache.NewMockRedisClient(t)
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, redisClient)

		// Redis RunScript fails, triggering fallback to DB.
		redisClient.EXPECT().
			RunScript(context.Background(), recordSuccessScript, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("redis unavailable")).
			Once()

		// Fallback path: getCircuitState calls stateCache.GetCircuitState
		// which also fails (redis is unavailable).
		redisClient.EXPECT().
			HGetAll(context.Background(), "aigateway:availability:circuit:1").
			Return(nil, errors.New("redis unavailable")).
			Maybe()

		// persistCircuitState → SetCircuitState (best-effort, may fail silently).
		redisClient.EXPECT().
			HMSet(context.Background(), "aigateway:availability:circuit:1", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Maybe()
		redisClient.EXPECT().
			HDel(context.Background(), "aigateway:availability:circuit:1", mock.Anything).
			Return(nil).Maybe()
		redisClient.EXPECT().
			Expire(context.Background(), "aigateway:availability:circuit:1", mock.Anything).
			Return(nil).Maybe()

		// No state in store → getCircuitState returns "not found" → default closed.
		err := cb.RecordSuccess(context.Background(), int64(1))
		require.NoError(t, err)

		// Fallback creates a default closed state with SuccessCount=1.
		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateClosed), dbState.CircuitState)
		require.Equal(t, 1, dbState.SuccessCount)
	})
}

// TestCircuitBreaker_LocalCacheTTL verifies the local cache expiration
// guarantees by confirming that cache entries expire after TTL and that
// the TTL ordering invariant (local < transition < redis) holds.
func TestCircuitBreaker_LocalCacheTTL(t *testing.T) {
	t.Run("ttl ordering invariant holds", func(t *testing.T) {
		require.Less(t, circuitStateLocalCacheTTL, circuitTransitionInterval,
			"localCache TTL MUST be less than transition interval to prevent deadlock")
		require.Less(t, circuitTransitionInterval, circuitStateCacheTTL,
			"transition interval MUST be less than Redis TTL for state survival")
	})

	t.Run("cache entry expires after local ttl", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil).(*circuitBreakerImpl)

		// Set a cache entry with a custom, short TTL.
		status := &types.ProviderCircuitStatus{
			UpstreamID:   int64(1),
			CircuitState: types.CircuitStateHalfOpen,
		}
		cacheKey := cb.localCacheKey(1)
		cb.mu.Lock()
		cb.localCache[cacheKey] = localCircuitCacheEntry{
			status:    status,
			expiresAt: time.Now().Add(10 * time.Millisecond),
		}
		cb.mu.Unlock()

		// Immediately, cache hit should work.
		cached := cb.getLocalCircuitCache(cacheKey)
		require.NotNil(t, cached)
		require.Equal(t, types.CircuitStateHalfOpen, cached.CircuitState)

		// After TTL expires, cache miss.
		time.Sleep(20 * time.Millisecond)
		expired := cb.getLocalCircuitCache(cacheKey)
		require.Nil(t, expired, "cache entry should expire after TTL")
	})
}

// TestCircuitBreaker_RecordFailure_FallbackPath tests the DB fallback path
// (stateCache disabled) where 3 consecutive RecordFailure calls cause the
// circuit to transition from closed to open via FailureCount accumulation.
// This covers circuit_breaker.go:238-262.
func TestCircuitBreaker_RecordFailure_FallbackPath(t *testing.T) {
	t.Run("three consecutive failures trigger open via db", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			OpenDuration:     30 * time.Second,
		}, store, nil) // nil redisClient → stateCache disabled → DB fallback

		// Call 1: FailureCount=1, still closed.
		err := cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err1"))
		require.NoError(t, err)
		s1, _ := store.GetByUpstreamID(context.Background(), int64(1))
		require.Equal(t, string(types.CircuitStateClosed), s1.CircuitState)
		require.Equal(t, 1, s1.FailureCount)

		// Call 2: FailureCount=2, still closed.
		err = cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err2"))
		require.NoError(t, err)
		s2, _ := store.GetByUpstreamID(context.Background(), int64(1))
		require.Equal(t, string(types.CircuitStateClosed), s2.CircuitState)
		require.Equal(t, 2, s2.FailureCount)

		// Call 3: FailureCount=3 >= threshold → transitions to open.
		err = cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err3"))
		require.NoError(t, err)
		s3, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateOpen), s3.CircuitState)
		require.Equal(t, 0, s3.FailureCount) // reset on open
		require.NotNil(t, s3.NextRetryAt)
	})

	t.Run("custom threshold of 5 requires 5 failures to open", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			OpenDuration:     30 * time.Second,
		}, store, nil)

		// Calls 1-4 should stay closed.
		for i := 0; i < 4; i++ {
			err := cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err"))
			require.NoError(t, err)
		}
		s4, _ := store.GetByUpstreamID(context.Background(), int64(1))
		require.Equal(t, string(types.CircuitStateClosed), s4.CircuitState)
		require.Equal(t, 4, s4.FailureCount)

		// Call 5: FailureCount=5 >= threshold → open.
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err5")))
		s5, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateOpen), s5.CircuitState)
	})

	t.Run("success resets failure count preventing open", func(t *testing.T) {
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			OpenDuration:     30 * time.Second,
		}, store, nil)

		// 2 failures.
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err1")))
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err2")))

		// A success resets FailureCount to 0.
		require.NoError(t, cb.RecordSuccess(context.Background(), int64(1)))
		sAfterSuccess, _ := store.GetByUpstreamID(context.Background(), int64(1))
		require.Equal(t, 0, sAfterSuccess.FailureCount)
		require.Equal(t, string(types.CircuitStateClosed), sAfterSuccess.CircuitState)

		// 3 more failures should open (fresh count, not accumulating old + new).
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err3")))
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err4")))
		require.NoError(t, cb.RecordFailure(context.Background(), int64(1), "test-model", errors.New("err5")))
		sFinal, _ := store.GetByUpstreamID(context.Background(), int64(1))
		require.Equal(t, string(types.CircuitStateOpen), sFinal.CircuitState)
	})
}

// TestCircuitBreaker_IsAvailable_StaleLocalCache verifies that the explicit
// invalidateLocalCache calls at circuit_breaker.go:132 and :142 prevent the
// deadlock regardless of how TTL constants are configured.
//
// Even if circuitStateLocalCacheTTL were much larger than
// circuitTransitionInterval (e.g., 100s > 5s), the logic still works because
// invalidateLocalCache actively removes the stale entry instead of waiting for
// TTL-based expiry. This proves the fix is robust and not dependent on TTL
// ordering.
func TestCircuitBreaker_IsAvailable_StaleLocalCache(t *testing.T) {
	t.Run("invalidateLocalCache prevents deadlock when localCacheTTL > transitionInterval", func(t *testing.T) {
		redisClient := mockcache.NewMockRedisClient(t)
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:      true,
			OpenDuration: 30 * time.Second,
		}, store, redisClient).(*circuitBreakerImpl)

		pastRetryTime := time.Now().Add(-time.Hour) // next retry was 1 hour ago

		// Seed localCache with a stale "open" state whose TTL (100s) far exceeds
		// circuitTransitionInterval (5s). Without invalidateLocalCache, this
		// would cause the deadlock described in the timing constants comment.
		cb.mu.Lock()
		cb.localCache[cb.localCacheKey(1)] = localCircuitCacheEntry{
			status: &types.ProviderCircuitStatus{
				UpstreamID:   int64(1),
				CircuitState: types.CircuitStateOpen,
				NextRetryAt:  &pastRetryTime,
			},
			expiresAt: time.Now().Add(100 * time.Second),
		}
		cb.mu.Unlock()

		// Another goroutine already transitioned Redis to half_open:
		// TryTransitionToHalfOpen returns false.
		redisClient.EXPECT().
			RunScript(mock.Anything, transitionToHalfOpenScript, mock.Anything, mock.Anything, mock.Anything).
			Return(int64(0), nil).
			Once()

		// After invalidateLocalCache at line 142, getCircuitState reads fresh
		// from Redis and finds half_open (not the stale open from cache).
		redisClient.EXPECT().
			HGetAll(mock.Anything, "aigateway:availability:circuit:1").
			Return(map[string]string{
				"circuit_state":     "half_open",
				"failure_count":     "0",
				"success_count":     "0",
				"last_state_change": fmt.Sprintf("%d", time.Now().Unix()),
			}, nil).
			Once()

		available, err := cb.IsAvailable(context.Background(), int64(1))
		require.NoError(t, err)
		require.True(t, available,
			"should be available: localCache was invalidated, fresh Redis read returned half_open")
	})

	t.Run("transitioned=true path also invalidates cache and reads fresh state", func(t *testing.T) {
		redisClient := mockcache.NewMockRedisClient(t)
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:      true,
			OpenDuration: 30 * time.Second,
		}, store, redisClient).(*circuitBreakerImpl)

		pastRetryTime := time.Now().Add(-time.Hour)
		nowUnix := time.Now().Unix()

		// Seed localCache with stale open state (TTL=100s > transitionInterval=5s).
		cb.mu.Lock()
		cb.localCache[cb.localCacheKey(1)] = localCircuitCacheEntry{
			status: &types.ProviderCircuitStatus{
				UpstreamID:   int64(1),
				CircuitState: types.CircuitStateOpen,
				NextRetryAt:  &pastRetryTime,
			},
			expiresAt: time.Now().Add(100 * time.Second),
		}
		cb.mu.Unlock()

		// This goroutine wins the transition: TryTransitionToHalfOpen returns true.
		redisClient.EXPECT().
			RunScript(mock.Anything, transitionToHalfOpenScript, mock.Anything, mock.Anything, mock.Anything).
			Return(int64(1), nil).
			Once()

		// After invalidateLocalCache at line 132, getCircuitState reads fresh
		// from Redis and finds half_open.
		redisClient.EXPECT().
			HGetAll(mock.Anything, "aigateway:availability:circuit:1").
			Return(map[string]string{
				"circuit_state":     "half_open",
				"failure_count":     "0",
				"success_count":     "0",
				"last_state_change": fmt.Sprintf("%d", nowUnix),
			}, nil).
			Once()

		// persistCircuitState → SetCircuitState → HMSet + HDel + Expire
		redisClient.EXPECT().
			HMSet(mock.Anything, "aigateway:availability:circuit:1", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		redisClient.EXPECT().
			HDel(mock.Anything, "aigateway:availability:circuit:1", "next_retry_at").
			Return(nil).Once()
		redisClient.EXPECT().
			Expire(mock.Anything, "aigateway:availability:circuit:1", 10*time.Second).
			Return(nil).Once()

		available, err := cb.IsAvailable(context.Background(), int64(1))
		require.NoError(t, err)
		require.True(t, available,
			"should be available: transitioned=true path invalidated cache and read fresh half_open")

		// Verify half_open persisted to DB.
		dbState, err := store.GetByUpstreamID(context.Background(), int64(1))
		require.NoError(t, err)
		require.Equal(t, string(types.CircuitStateHalfOpen), dbState.CircuitState)
	})

	t.Run("stale open cache with expired NextRetryAt and no redis leads to unavailability", func(t *testing.T) {
		redisClient := mockcache.NewMockRedisClient(t)
		store := newFakeCircuitStore()
		cb := NewCircuitBreaker(types.CircuitBreakerConfig{
			Enabled:      true,
			OpenDuration: 30 * time.Second,
		}, store, redisClient).(*circuitBreakerImpl)

		pastRetryTime := time.Now().Add(-time.Hour)

		// Seed localCache with open state, NextRetryAt in the past.
		cb.mu.Lock()
		cb.localCache[cb.localCacheKey(1)] = localCircuitCacheEntry{
			status: &types.ProviderCircuitStatus{
				UpstreamID:   int64(1),
				CircuitState: types.CircuitStateOpen,
				NextRetryAt:  &pastRetryTime,
			},
			expiresAt: time.Now().Add(100 * time.Second),
		}
		cb.mu.Unlock()

		// TryTransitionToHalfOpen returns false (another goroutine attempted).
		redisClient.EXPECT().
			RunScript(mock.Anything, transitionToHalfOpenScript, mock.Anything, mock.Anything, mock.Anything).
			Return(int64(0), nil).
			Once()

		// After invalidation, Redis still returns open (the other goroutine
		// could not transition because NextRetryAt had not passed yet in Redis,
		// or the circuit had already been re-opened).
		redisClient.EXPECT().
			HGetAll(mock.Anything, "aigateway:availability:circuit:1").
			Return(map[string]string{
				"circuit_state":     "open",
				"failure_count":     "0",
				"success_count":     "0",
				"last_state_change": fmt.Sprintf("%d", pastRetryTime.Unix()),
				"next_retry_at":     fmt.Sprintf("%d", time.Now().Add(30*time.Second).Unix()),
			}, nil).
			Once()

		available, err := cb.IsAvailable(context.Background(), int64(1))
		require.NoError(t, err)
		require.False(t, available,
			"should be unavailable: fresh Redis read still shows open with future NextRetryAt")
	})
}

func TestCircuitStateToGaugeValue(t *testing.T) {
	tests := []struct {
		name     string
		state    types.CircuitState
		expected float64
	}{
		{"open state", types.CircuitStateOpen, 0},
		{"half_open state", types.CircuitStateHalfOpen, 1},
		{"closed state", types.CircuitStateClosed, 2},
		{"unknown state defaults to closed value", types.CircuitState("unknown"), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, circuitStateToGaugeValue(tt.state))
		})
	}
}

// TestCircuitBreaker_PersistCircuitState_MetricReporting verifies that
// persistCircuitState correctly sets the AIGatewayUpstreamCircuitState
// Prometheus GaugeVec metric. It uses a fresh registry (the same gatherer
// interface that promhttp.HandlerFor consumers use) to scrape and assert
// the metric values end-to-end.
func TestCircuitBreaker_PersistCircuitState_MetricReporting(t *testing.T) {
	// Create a fresh registry so we don't pollute the global default registry.
	reg := prometheus.NewRegistry()
	metric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_upstream_circuit_state",
		Help: "Circuit breaker state of aigateway upstreams (0=open, 1=half_open, 2=closed)",
	}, []string{"upstream_id", "model_name", "provider", "circuit_state"})
	reg.MustRegister(metric)

	// Swap the global metric with our isolated one; restore after the test.
	origMetric := prom.AIGatewayUpstreamCircuitState
	prom.AIGatewayUpstreamCircuitState = metric
	defer func() { prom.AIGatewayUpstreamCircuitState = origMetric }()

	store := newFakeCircuitStore()
	cb := NewCircuitBreaker(types.CircuitBreakerConfig{Enabled: true}, store, nil).(*circuitBreakerImpl)

	now := time.Now()
	nextRetry := now.Add(30 * time.Second)

	// ---- open ----
	statusOpen := &types.ProviderCircuitStatus{
		UpstreamID:      42,
		ModelName:       "gpt-4",
		Provider:        "OpenAI",
		CircuitState:    types.CircuitStateOpen,
		FailureCount:    3,
		SuccessCount:    0,
		LastStateChange: now,
		NextRetryAt:     &nextRetry,
	}
	require.NoError(t, cb.persistCircuitState(context.Background(), statusOpen))
	assertCircuitMetricValue(t, reg, "42", "gpt-4", "OpenAI", "open", 0)

	// ---- half_open ----
	metric.Reset()
	statusHalfOpen := &types.ProviderCircuitStatus{
		UpstreamID:      42,
		ModelName:       "gpt-4",
		Provider:        "OpenAI",
		CircuitState:    types.CircuitStateHalfOpen,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: now,
	}
	require.NoError(t, cb.persistCircuitState(context.Background(), statusHalfOpen))
	assertCircuitMetricValue(t, reg, "42", "gpt-4", "OpenAI", "half_open", 1)

	// ---- closed ----
	metric.Reset()
	statusClosed := &types.ProviderCircuitStatus{
		UpstreamID:      42,
		ModelName:       "gpt-4",
		Provider:        "OpenAI",
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    1,
		LastStateChange: now,
	}
	require.NoError(t, cb.persistCircuitState(context.Background(), statusClosed))
	assertCircuitMetricValue(t, reg, "42", "gpt-4", "OpenAI", "closed", 2)
}

func assertCircuitMetricValue(t *testing.T, reg *prometheus.Registry, upstreamID, modelName, provider, circuitState string, want float64) {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, families, 1, "expect exactly one metric family")

	family := families[0]
	require.Equal(t, "csghub_aigateway_upstream_circuit_state", family.GetName())
	require.Len(t, family.GetMetric(), 1, "expect exactly one metric")

	m := family.GetMetric()[0]
	require.Equal(t, want, m.GetGauge().GetValue(), "unexpected gauge value for state %q", circuitState)

	gotLabels := make(map[string]string)
	for _, l := range m.GetLabel() {
		gotLabels[l.GetName()] = l.GetValue()
	}
	require.Equal(t, upstreamID, gotLabels["upstream_id"])
	require.Equal(t, modelName, gotLabels["model_name"])
	require.Equal(t, provider, gotLabels["provider"])
	require.Equal(t, circuitState, gotLabels["circuit_state"])
	require.Equal(t, "", gotLabels["url"], "url label should not exist")
}
