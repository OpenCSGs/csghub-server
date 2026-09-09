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

	state, err := cache.RecordFailure(context.Background(), types.StateCacheRecordInput{
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

func TestStateCache_GetMultimodalProbeState(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)
	nextInferenceAt := time.Unix(1700003600, 0)

	redisClient.EXPECT().
		HGetAll(context.Background(), "aigateway:availability:multimodal-probe:1").
		Return(map[string]string{
			"mode":                 "inference_only",
			"next_inference_at":    "1700003600",
			"consecutive_failures": "2",
			"l7_failures":          "1",
		}, nil).
		Once()

	state, err := cache.GetMultimodalProbeState(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, multimodalProbeInferenceOnly, state.mode)
	require.Equal(t, nextInferenceAt, state.nextInferenceAt)
}

func TestStateCache_GetMultimodalProbeState_IgnoresLegacyHealthFields(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)

	redisClient.EXPECT().
		HGetAll(context.Background(), "aigateway:availability:multimodal-probe:1").
		Return(map[string]string{
			"mode":                 "l7_available",
			"next_inference_at":    "1700003600",
			"consecutive_failures": "0",
		}, nil).
		Once()

	state, err := cache.GetMultimodalProbeState(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, multimodalProbeL7Available, state.mode)
	require.Equal(t, time.Unix(1700003600, 0), state.nextInferenceAt)
}

func TestStateCache_GetMultimodalProbeState_Miss(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)

	redisClient.EXPECT().
		HGetAll(context.Background(), "aigateway:availability:multimodal-probe:1").
		Return(map[string]string{}, nil).
		Once()

	_, err := cache.GetMultimodalProbeState(context.Background(), 1)
	require.ErrorIs(t, err, errStateCacheMiss)
}

func TestStateCache_GetMultimodalProbeState_RejectsMalformedState(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)

	redisClient.EXPECT().
		HGetAll(context.Background(), "aigateway:availability:multimodal-probe:1").
		Return(map[string]string{
			"mode":                 "unsupported",
			"next_inference_at":    "1700003600",
			"consecutive_failures": "0",
		}, nil).
		Once()

	_, err := cache.GetMultimodalProbeState(context.Background(), 1)
	require.ErrorContains(t, err, "invalid multimodal probe mode")
}

func TestStateCache_SetMultimodalProbeState(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)
	state := multimodalProbeState{
		mode:            multimodalProbeL7Available,
		nextInferenceAt: time.Unix(1700003600, 0),
	}

	redisClient.EXPECT().
		RunScript(
			context.Background(),
			setMultimodalProbeStateScript,
			[]string{"aigateway:availability:multimodal-probe:1"},
			"l7_available",
			int64(1700003600),
			7200,
		).
		Return(int64(1), nil).
		Once()

	require.NoError(t, cache.SetMultimodalProbeState(context.Background(), 1, state, 2*time.Hour))
}

func TestStateCache_TryReserveMultimodalInference(t *testing.T) {
	for _, test := range []struct {
		name     string
		reserved int64
		expected bool
	}{
		{name: "claimed", reserved: 1, expected: true},
		{name: "already reserved", reserved: 0, expected: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			redisClient := mockcache.NewMockRedisClient(t)
			cache := NewStateCache(redisClient).(multimodalProbeStateCache)
			now := time.Unix(1700000000, 0)
			reservedUntil := now.Add(6 * time.Minute)
			state := multimodalProbeState{mode: multimodalProbeInferenceOnly}

			redisClient.EXPECT().
				RunScript(
					context.Background(),
					reserveMultimodalInferenceScript,
					[]string{"aigateway:availability:multimodal-probe:1"},
					now.Unix(),
					reservedUntil.Unix(),
					"inference_only",
					7200,
				).
				Return([]any{test.reserved, "inference_only", reservedUntil.Unix()}, nil).
				Once()

			reservedState, reserved, err := cache.TryReserveMultimodalInference(
				context.Background(), 1, state, now, reservedUntil, 2*time.Hour,
			)
			require.NoError(t, err)
			require.Equal(t, test.expected, reserved)
			require.Equal(t, multimodalProbeInferenceOnly, reservedState.mode)
			require.Equal(t, reservedUntil, reservedState.nextInferenceAt)
		})
	}
}

func TestStateCache_DeleteMultimodalProbeState(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	cache := NewStateCache(redisClient).(multimodalProbeStateCache)

	redisClient.EXPECT().
		Del(context.Background(), "aigateway:availability:multimodal-probe:1").
		Return(nil).
		Once()

	require.NoError(t, cache.DeleteMultimodalProbeState(context.Background(), 1))
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
