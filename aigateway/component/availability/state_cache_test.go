package availability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/types"
)

var errTestSentinel = errors.New("test error")

func TestStateCache_TryTransitionToHalfOpen(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)

	redisClient.EXPECT().
		RunScript(context.Background(), transitionToHalfOpenScript, []string{
			"aigateway:availability:circuit:1",
			"aigateway:availability:circuit:half-open:1",
		}, mock.Anything, int((10*time.Second).Seconds())).
		Return(int64(1), nil).
		Once()

	ok, err := cache.TryTransitionToHalfOpen(context.Background(), int64(1), time.Now(), 10*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestStateCache_TryAcquireHalfOpenSlot(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)

	redisClient.EXPECT().
		RunScript(context.Background(), incrementHalfOpenRequestsScript, []string{
			"aigateway:availability:circuit:half-open:1",
		}, 1, int((10*time.Second).Seconds())).
		Return([]any{int64(0), int64(1)}, nil).
		Once()

	allowed, current, err := cache.TryAcquireHalfOpenSlot(context.Background(), int64(1), 1, 10*time.Second)
	require.NoError(t, err)
	require.False(t, allowed)
	require.EqualValues(t, 1, current)
}

func TestStateCache_RecordFailure(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	now := time.Unix(1700000000, 0)

	redisClient.EXPECT().
		RunScript(context.Background(), recordFailureScript, []string{
			"aigateway:availability:circuit:1",
			"aigateway:availability:circuit:half-open:1",
		}, 3, int((30*time.Second).Seconds()), now.Unix(), int((10*time.Second).Seconds())).
		Return([]any{"open", int64(0), int64(0), now.Unix(), now.Add(30 * time.Second).Unix()}, nil).
		Once()

	state, err := cache.RecordFailure(context.Background(), stateCacheRecordInput{
		UpstreamID: int64(1),
		Now:        now,
		TTL:        10 * time.Second,
	}, 3, 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, types.CircuitStateOpen, state.CircuitState)
	require.NotNil(t, state.NextRetryAt)
}

func TestStateCache_HealthStateRoundtrip(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderHealthStatus{
		UpstreamID:  1,
		Provider:    "openai",
		ModelName:   "gpt-4",
		Endpoint:    "https://api.example.com",
		HealthState: types.HealthStateHealthy,
		LastCheckAt: time.Unix(1700000000, 0),
	}

	redisClient.EXPECT().
		SetEx(context.Background(), "aigateway:availability:health:1", mock.Anything, 30*time.Second).
		RunAndReturn(func(_ context.Context, _ string, payload string, _ time.Duration) error {
			require.Contains(t, payload, "\"provider\":\"openai\"")
			return nil
		}).
		Once()

	err := cache.SetHealthState(context.Background(), state, 30*time.Second)
	require.NoError(t, err)
}

func TestStateCache_SetCircuitState_WithNextRetryAt(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	retryAt := time.Unix(1700000030, 0)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateOpen,
		FailureCount:    3,
		SuccessCount:    0,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     &retryAt,
	}

	redisClient.EXPECT().
		HMSet(context.Background(), "aigateway:availability:circuit:1",
			"circuit_state", "open",
			"failure_count", 3,
			"success_count", 0,
			"last_state_change", int64(1700000000),
			"next_retry_at", int64(1700000030),
		).
		Return(nil).
		Once()
	redisClient.EXPECT().
		Expire(context.Background(), "aigateway:availability:circuit:1", 10*time.Second).
		Return(nil).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 10*time.Second)
	require.NoError(t, err)
}

func TestStateCache_SetCircuitState_WithoutNextRetryAt(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    5,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     nil,
	}

	redisClient.EXPECT().
		HMSet(context.Background(), "aigateway:availability:circuit:1",
			"circuit_state", "closed",
			"failure_count", 0,
			"success_count", 5,
			"last_state_change", int64(1700000000),
		).
		Return(nil).
		Once()
	redisClient.EXPECT().
		HDel(context.Background(), "aigateway:availability:circuit:1", "next_retry_at").
		Return(nil).
		Once()
	redisClient.EXPECT().
		Expire(context.Background(), "aigateway:availability:circuit:1", 10*time.Second).
		Return(nil).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 10*time.Second)
	require.NoError(t, err)
}

func TestStateCache_SetCircuitState_StateIsNil(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)

	err := cache.SetCircuitState(context.Background(), nil, 10*time.Second)
	require.NoError(t, err)
}

func TestStateCache_SetCircuitState_HMSetError(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     nil,
	}

	redisClient.EXPECT().
		HMSet(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errTestSentinel).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 10*time.Second)
	require.Error(t, err)
}

func TestStateCache_SetCircuitState_HDelError(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     nil,
	}

	redisClient.EXPECT().
		HMSet(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().
		HDel(mock.Anything, mock.Anything, mock.Anything).
		Return(errTestSentinel).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 10*time.Second)
	require.Error(t, err)
}

func TestStateCache_SetCircuitState_ExpireError(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     nil,
	}

	redisClient.EXPECT().
		HMSet(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().
		HDel(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().
		Expire(mock.Anything, mock.Anything, mock.Anything).
		Return(errTestSentinel).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 10*time.Second)
	require.Error(t, err)
}

func TestStateCache_SetCircuitState_DefaultTTL(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient)
	state := &types.ProviderCircuitStatus{
		UpstreamID:      1,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: time.Unix(1700000000, 0),
		NextRetryAt:     nil,
	}

	redisClient.EXPECT().
		HMSet(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().
		HDel(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	redisClient.EXPECT().
		Expire(context.Background(), "aigateway:availability:circuit:1", 30*time.Second).
		Return(nil).
		Once()

	err := cache.SetCircuitState(context.Background(), state, 0)
	require.NoError(t, err)
}
