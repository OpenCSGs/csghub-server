package availability

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/cache"
)

// These tests exercise the real Lua record/transition scripts against
// miniredis. They guard the invariant that a record (success or failure)
// observed while the circuit is open — an in-flight request finishing after
// the trip — must preserve next_retry_at. Wiping it strands the circuit open
// with no eligible retry time: IsAvailable blocks on it and
// transitionToHalfOpenScript used to refuse a missing deadline.

func newScriptTestStateCache(t *testing.T) (StateCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	redisClient := cache.NewCacheWithClient(context.Background(), redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	return NewStateCache(redisClient), mr
}

func seedCircuitState(t *testing.T, sc StateCache, upstreamID int64, state types.CircuitState, failureCount, successCount int, nextRetryAt *time.Time) {
	t.Helper()
	status := &types.ProviderCircuitStatus{
		UpstreamID:      upstreamID,
		CircuitState:    state,
		FailureCount:    failureCount,
		SuccessCount:    successCount,
		LastStateChange: time.Now(),
		NextRetryAt:     nextRetryAt,
	}
	require.NoError(t, sc.SetCircuitState(context.Background(), status, circuitStateCacheTTL))
}

func getCachedCircuitState(t *testing.T, sc StateCache, upstreamID int64) *types.ProviderCircuitStatus {
	t.Helper()
	status, err := sc.GetCircuitState(context.Background(), upstreamID)
	require.NoError(t, err)
	return status
}

func TestStateCacheScripts_RecordSuccessWhileOpen_PreservesNextRetryAt(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	nextRetry := time.Now().Add(30 * time.Second)
	seedCircuitState(t, sc, 1, types.CircuitStateOpen, 0, 0, &nextRetry)

	status, err := sc.RecordSuccess(context.Background(), types.StateCacheRecordInput{
		UpstreamID: 1,
		Now:        time.Now(),
		TTL:        circuitStateCacheTTL,
	})
	require.NoError(t, err)
	require.Equal(t, types.CircuitStateOpen, status.CircuitState, "success while open must not change the state")
	require.Equal(t, 1, status.SuccessCount)
	require.NotNil(t, status.NextRetryAt, "success while open must preserve next_retry_at")
	require.WithinDuration(t, nextRetry, *status.NextRetryAt, time.Second)

	cached := getCachedCircuitState(t, sc, 1)
	require.Equal(t, types.CircuitStateOpen, cached.CircuitState)
	require.NotNil(t, cached.NextRetryAt)
}

func TestStateCacheScripts_RecordFailureWhileOpen_PreservesNextRetryAt(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	nextRetry := time.Now().Add(30 * time.Second)
	seedCircuitState(t, sc, 1, types.CircuitStateOpen, 0, 0, &nextRetry)

	status, err := sc.RecordFailure(context.Background(), types.StateCacheRecordInput{
		UpstreamID: 1,
		Now:        time.Now(),
		TTL:        circuitStateCacheTTL,
	}, 3, 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, types.CircuitStateOpen, status.CircuitState, "failure below threshold while open must stay open")
	require.Equal(t, 1, status.FailureCount)
	require.NotNil(t, status.NextRetryAt, "failure while open must preserve next_retry_at")
	require.WithinDuration(t, nextRetry, *status.NextRetryAt, time.Second)
}

func TestStateCacheScripts_RecordSuccess_HalfOpenClosesAndClearsRetry(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	seedCircuitState(t, sc, 1, types.CircuitStateHalfOpen, 0, 0, nil)

	status, err := sc.RecordSuccess(context.Background(), types.StateCacheRecordInput{
		UpstreamID: 1,
		Now:        time.Now(),
		TTL:        circuitStateCacheTTL,
	})
	require.NoError(t, err)
	require.Equal(t, types.CircuitStateClosed, status.CircuitState)
	require.Nil(t, status.NextRetryAt)
	require.Equal(t, 0, status.SuccessCount)
}

func TestStateCacheScripts_TryTransitionToHalfOpen_MissingNextRetryAt(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	// Simulates a legacy row whose next_retry_at was wiped while open: the
	// deadline is missing, so the circuit must still be eligible for the
	// half-open probe instead of staying open forever.
	seedCircuitState(t, sc, 1, types.CircuitStateOpen, 0, 6, nil)

	transitioned, err := sc.TryTransitionToHalfOpen(context.Background(), 1, time.Now(), circuitStateCacheTTL)
	require.NoError(t, err)
	require.True(t, transitioned)

	cached := getCachedCircuitState(t, sc, 1)
	require.Equal(t, types.CircuitStateHalfOpen, cached.CircuitState)
}

func TestStateCacheScripts_TryTransitionToHalfOpen_ElapsedRetry(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	seedCircuitState(t, sc, 1, types.CircuitStateOpen, 0, 0, ptrTime(time.Now().Add(-time.Minute)))

	transitioned, err := sc.TryTransitionToHalfOpen(context.Background(), 1, time.Now(), circuitStateCacheTTL)
	require.NoError(t, err)
	require.True(t, transitioned)
}

func TestStateCacheScripts_TryTransitionToHalfOpen_FutureRetryStaysOpen(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)

	seedCircuitState(t, sc, 1, types.CircuitStateOpen, 0, 0, ptrTime(time.Now().Add(time.Minute)))

	transitioned, err := sc.TryTransitionToHalfOpen(context.Background(), 1, time.Now(), circuitStateCacheTTL)
	require.NoError(t, err)
	require.False(t, transitioned)

	cached := getCachedCircuitState(t, sc, 1)
	require.Equal(t, types.CircuitStateOpen, cached.CircuitState)
	require.NotNil(t, cached.NextRetryAt, "rejected transition must not wipe next_retry_at")
}

// TestStateCacheScripts_OpenCircuitRecovery reproduces the incident lifecycle:
// the circuit opens, in-flight successes are recorded while open (which used
// to wipe next_retry_at), the retry window elapses, and the circuit must be
// able to reach half_open and then close again.
func TestStateCacheScripts_OpenCircuitRecovery(t *testing.T) {
	sc, _ := newScriptTestStateCache(t)
	ctx := context.Background()

	// 1. Open via threshold failures.
	var status *types.ProviderCircuitStatus
	var err error
	for i := 0; i < 3; i++ {
		status, err = sc.RecordFailure(ctx, types.StateCacheRecordInput{
			UpstreamID: 1,
			Now:        time.Now(),
			TTL:        circuitStateCacheTTL,
		}, 3, 30*time.Second)
		require.NoError(t, err)
	}
	require.Equal(t, types.CircuitStateOpen, status.CircuitState)
	require.NotNil(t, status.NextRetryAt, "open must carry a retry deadline")

	// 2. In-flight successes recorded while open must not wipe the deadline.
	for i := 0; i < 6; i++ {
		status, err = sc.RecordSuccess(ctx, types.StateCacheRecordInput{
			UpstreamID: 1,
			Now:        time.Now(),
			TTL:        circuitStateCacheTTL,
		})
		require.NoError(t, err)
	}
	require.Equal(t, types.CircuitStateOpen, status.CircuitState)
	require.Equal(t, 6, status.SuccessCount)
	require.NotNil(t, status.NextRetryAt, "successes while open must preserve next_retry_at")

	// 3. Deadline elapses; transition to half_open must succeed.
	transitioned, err := sc.TryTransitionToHalfOpen(ctx, 1, time.Now().Add(31*time.Second), circuitStateCacheTTL)
	require.NoError(t, err)
	require.True(t, transitioned)

	// 4. Half-open probe success closes the circuit.
	status, err = sc.RecordSuccess(ctx, types.StateCacheRecordInput{
		UpstreamID: 1,
		Now:        time.Now().Add(31 * time.Second),
		TTL:        circuitStateCacheTTL,
	})
	require.NoError(t, err)
	require.Equal(t, types.CircuitStateClosed, status.CircuitState)
	require.Nil(t, status.NextRetryAt)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
