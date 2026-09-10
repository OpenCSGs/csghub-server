package availability

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockavailability "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component/availability"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/sample"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

// mockTransport provides a configurable RoundTripper for HTTP tests.
type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

type executingSampleProvider struct {
	executionPolicy func(types.SampleKind) (types.SampleExecutionPolicy, error)
	execute         func(context.Context, types.SampleKind, types.SampleInput, types.HTTPDoer) (*types.SampleExecutionResult, error)
}

func (p executingSampleProvider) Supports(string) bool {
	return true
}

func (p executingSampleProvider) ExecutionPolicy(kind types.SampleKind) (types.SampleExecutionPolicy, error) {
	if p.executionPolicy != nil {
		return p.executionPolicy(kind)
	}
	return types.SampleExecutionPolicy{Timeout: 30 * time.Second}, nil
}

func (p executingSampleProvider) Execute(ctx context.Context, kind types.SampleKind, input types.SampleInput, client types.HTTPDoer) (*types.SampleExecutionResult, error) {
	return p.execute(ctx, kind, input, client)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func expectStatefulMultimodalHealthStore(
	t *testing.T,
	store *mockdatabase.MockAIGatewayUpstreamHealthStateStore,
	updateCount int,
	initial ...*database.AIGatewayUpstreamHealthState,
) func() *database.AIGatewayUpstreamHealthState {
	t.Helper()
	var persisted *database.AIGatewayUpstreamHealthState
	if len(initial) > 0 {
		persisted = cloneHealthState(initial[0])
	}
	var mutex sync.Mutex
	store.EXPECT().MutateByUpstreamID(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, mutation database.AIGatewayUpstreamHealthStateMutation) (*database.AIGatewayUpstreamHealthState, error) {
			mutex.Lock()
			defer mutex.Unlock()
			if persisted == nil {
				if !mutation.CreateIfMissing {
					return nil, sql.ErrNoRows
				}
				persisted = &database.AIGatewayUpstreamHealthState{
					UpstreamID:  mutation.UpstreamID,
					HealthState: string(types.HealthStateHealthy),
				}
			}
			copied := cloneHealthState(persisted)
			if err := mutation.Mutate(copied); err != nil {
				return nil, err
			}
			persisted = copied
			return cloneHealthState(copied), nil
		}).
		Times(updateCount)
	return func() *database.AIGatewayUpstreamHealthState {
		mutex.Lock()
		defer mutex.Unlock()
		return cloneHealthState(persisted)
	}
}

func cloneHealthState(state *database.AIGatewayUpstreamHealthState) *database.AIGatewayUpstreamHealthState {
	if state == nil {
		return nil
	}
	cloned := *state
	if state.Metadata != nil {
		cloned.Metadata = make(map[string]any, len(state.Metadata))
		for key, value := range state.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return &cloned
}

func TestHealthChecker_UpdateHealthState_Degraded(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1)

	checker := &healthCheckerImpl{
		circuitBreaker: nil,
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
					LatencyThresholdForDegraded:     2 * time.Second,
				},
			},
		},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1,
		ModelName:  "gpt-4",
		Endpoint:   "https://api.example.com",
		Healthy:    true,
		LatencyMs:  3000,
		Timestamp:  time.Now(),
	})

	require.Equal(t, string(types.HealthStateDegraded), latestState().HealthState)
}

func TestHealthChecker_UpdateHealthState_MultimodalLatencyThreshold(t *testing.T) {
	tests := []struct {
		name          string
		latencyMs     int64
		expectedState types.HealthState
	}{
		{
			name:          "latency below multimodal threshold stays healthy",
			latencyMs:     40733,
			expectedState: types.HealthStateHealthy,
		},
		{
			name:          "latency equal to multimodal threshold stays healthy",
			latencyMs:     120000,
			expectedState: types.HealthStateHealthy,
		},
		{
			name:          "latency above multimodal threshold becomes degraded",
			latencyMs:     120001,
			expectedState: types.HealthStateDegraded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
			latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1)

			checker := &healthCheckerImpl{
				config: HealthCheckerConfig{Config: types.HealthCheckConfig{
					HealthRules: types.HealthRulesConfig{
						ConsecutiveFailuresForUnhealthy:       3,
						LatencyThresholdForDegraded:           10 * time.Second,
						MultimodalLatencyThresholdForDegraded: 2 * time.Minute,
					},
				}},
				healthStore: mockStore,
				stateCache:  NewStateCache(nil),
			}

			checker.updateHealthState(context.Background(), &types.HealthCheckResult{
				UpstreamID: 1,
				ModelName:  "image-model",
				CheckType:  types.HealthCheckTypeInference,
				Healthy:    true,
				LatencyMs:  test.latencyMs,
				Timestamp:  time.Now(),
				Multimodal: true,
			})

			require.Equal(t, string(test.expectedState), latestState().HealthState)
		})
	}
}

func TestHealthChecker_UpdateMultimodalHealthStateKeepsProbeFailuresIndependent(t *testing.T) {
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	latestState := expectStatefulMultimodalHealthStore(t, store, 4)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			HealthRules: types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore: store,
		stateCache:  NewStateCache(nil),
	}
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	events := []*types.HealthCheckResult{
		{UpstreamID: 1, CheckType: types.HealthCheckTypeInference, Error: "inference 1", Timestamp: start, Multimodal: true},
		{UpstreamID: 1, CheckType: types.HealthCheckTypeL7API, Error: "l7", Timestamp: start.Add(time.Minute), Multimodal: true},
		{UpstreamID: 1, CheckType: types.HealthCheckTypeL7API, Healthy: true, Timestamp: start.Add(2 * time.Minute), Multimodal: true},
		{UpstreamID: 1, CheckType: types.HealthCheckTypeInference, Error: "inference 2", Timestamp: start.Add(3 * time.Minute), Multimodal: true},
	}
	for _, event := range events {
		require.True(t, checker.updateHealthState(context.Background(), event))
	}

	persisted := latestState()
	require.NotNil(t, persisted)
	health, found, err := readMultimodalHealthState(persisted.Metadata)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, health.L7.ConsecutiveFailures)
	require.Equal(t, 2, health.Inference.ConsecutiveFailures)
	require.Equal(t, 2, persisted.ConsecutiveFailures)
	require.Equal(t, "inference 2", persisted.LastError)
	require.Equal(t, string(types.HealthStateHealthy), persisted.HealthState)
}

func TestHealthChecker_UpdateMultimodalHealthStateRejectsDelayedOlderEvent(t *testing.T) {
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	latestState := expectStatefulMultimodalHealthStore(t, store, 2)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			HealthRules: types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore: store,
		stateCache:  NewStateCache(nil),
	}
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	oldResult := make(chan bool, 1)

	go func() {
		close(oldStarted)
		<-releaseOld
		oldResult <- checker.updateHealthState(context.Background(), &types.HealthCheckResult{
			UpstreamID: 1, CheckType: types.HealthCheckTypeL7API, Error: "old failure", Timestamp: start, Multimodal: true,
		})
	}()
	<-oldStarted
	require.True(t, checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1, CheckType: types.HealthCheckTypeL7API, Healthy: true, LatencyMs: 50,
		Timestamp: start.Add(time.Minute), Multimodal: true,
	}))
	close(releaseOld)
	require.False(t, <-oldResult)

	persisted := latestState()
	health, found, err := readMultimodalHealthState(persisted.Metadata)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, start.Add(time.Minute), health.L7.LastCheckedAt)
	require.Zero(t, health.L7.ConsecutiveFailures)
	require.Empty(t, health.L7.LastError)
	require.Equal(t, int64(50), health.L7.LatencyMs)
}

func TestHealthChecker_UpdateHealthStateRejectsStaleRegularResultWithoutSideEffects(t *testing.T) {
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	checkedAt := time.Date(2026, 8, 24, 10, 1, 0, 0, time.UTC)
	initial := &database.AIGatewayUpstreamHealthState{
		ID: 10, UpstreamID: 1, HealthState: string(types.HealthStateUnhealthy),
		LastCheckAt: checkedAt, ConsecutiveFailures: 3, LastError: "new failure", LatencyMs: 100,
	}
	latestState := expectStatefulMultimodalHealthStore(t, store, 2, initial)
	stateCache := mockavailability.NewMockStateCache(t)
	circuitBreaker := newMockCircuitBreaker()
	checker := &healthCheckerImpl{
		healthStore: store, stateCache: stateCache, circuitBreaker: circuitBreaker,
	}

	for _, staleCheckedAt := range []time.Time{checkedAt.Add(-time.Minute), checkedAt} {
		persisted := checker.updateHealthState(context.Background(), &types.HealthCheckResult{
			UpstreamID: 1, CheckType: types.HealthCheckTypeInference, Healthy: true,
			Timestamp: staleCheckedAt,
		})
		require.False(t, persisted)
	}
	require.Equal(t, initial, latestState())
	require.False(t, circuitBreaker.forceClosed)
}

func TestHealthChecker_UpdateMultimodalHealthStateLegacyRowStaysUnknownUntilInference(t *testing.T) {
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	legacy := &database.AIGatewayUpstreamHealthState{
		ID:                  10,
		UpstreamID:          1,
		HealthState:         string(types.HealthStateUnhealthy),
		ConsecutiveFailures: 3,
		LastError:           "legacy failure",
	}
	latestState := expectStatefulMultimodalHealthStore(t, store, 2, legacy)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			HealthRules: types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore: store,
		stateCache:  NewStateCache(nil),
	}
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

	require.True(t, checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1, CheckType: types.HealthCheckTypeL7API, Healthy: true, Timestamp: start, Multimodal: true,
	}))
	require.Equal(t, string(types.HealthStateUnknown), latestState().HealthState)

	require.True(t, checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1, CheckType: types.HealthCheckTypeInference, Healthy: true, Timestamp: start.Add(time.Minute), Multimodal: true,
	}))
	require.Equal(t, string(types.HealthStateHealthy), latestState().HealthState)
}

func TestHealthChecker_UpdateMultimodalHealthStateRejectsCorruptedMetadata(t *testing.T) {
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	corrupted := &database.AIGatewayUpstreamHealthState{
		ID:          10,
		UpstreamID:  1,
		HealthState: string(types.HealthStateUnhealthy),
		Metadata: map[string]any{
			multimodalHealthMetadataKey: map[string]any{"l7": map[string]any{}},
		},
	}
	latestState := expectStatefulMultimodalHealthStore(t, store, 1, corrupted)
	checker := &healthCheckerImpl{healthStore: store, stateCache: NewStateCache(nil)}

	persisted := checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1, CheckType: types.HealthCheckTypeInference, Healthy: true, Timestamp: time.Now(), Multimodal: true,
	})

	require.False(t, persisted)
	require.Equal(t, string(types.HealthStateUnhealthy), latestState().HealthState)
	require.Equal(t, corrupted.Metadata, latestState().Metadata)
}

func TestHealthChecker_UpdateHealthState_NilResult(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	checker := &healthCheckerImpl{
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), nil)
}

func TestHealthChecker_UpdateHealthState_NewUpstreamHealthy(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1)

	checker := &healthCheckerImpl{
		circuitBreaker: nil,
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
				},
			},
		},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1,
		ModelName:  "gpt-4",
		Provider:   "openai",
		Endpoint:   "https://api.example.com",
		CheckType:  types.HealthCheckTypeL7API,
		Healthy:    true,
		LatencyMs:  50,
		Timestamp:  time.Now(),
	})

	state := latestState()
	require.Equal(t, string(types.HealthStateHealthy), state.HealthState)
	require.Equal(t, int64(1), state.UpstreamID)
	require.Equal(t, int64(50), state.LatencyMs)
	require.Equal(t, 0, state.ConsecutiveFailures)
	require.Empty(t, state.LastError)
}

func TestHealthChecker_UpdateHealthState_NewUpstreamUnhealthy(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 3)

	checker := &healthCheckerImpl{
		circuitBreaker: nil,
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
				},
			},
		},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	for i := 0; i < 3; i++ {
		checker.updateHealthState(context.Background(), &types.HealthCheckResult{
			UpstreamID: 1,
			ModelName:  "gpt-4",
			Endpoint:   "https://api.example.com",
			CheckType:  types.HealthCheckTypeL7API,
			Healthy:    false,
			Error:      "connection refused",
			LatencyMs:  100,
			Timestamp:  time.Now(),
		})
	}

	finalState := latestState()
	require.Equal(t, string(types.HealthStateUnhealthy), finalState.HealthState)
	require.Equal(t, 3, finalState.ConsecutiveFailures)
	require.Equal(t, "connection refused", finalState.LastError)
}

func TestHealthChecker_UpdateHealthState_ExistingUpstreamRecovers(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	existingState := &database.AIGatewayUpstreamHealthState{
		ID:                  10,
		UpstreamID:          1,
		HealthState:         string(types.HealthStateUnhealthy),
		ConsecutiveFailures: 5,
		LastError:           "previous error",
	}
	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1, existingState)

	checker := &healthCheckerImpl{
		circuitBreaker: nil,
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
				},
			},
		},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1,
		ModelName:  "gpt-4",
		Endpoint:   "https://api.example.com",
		CheckType:  types.HealthCheckTypeL7API,
		Healthy:    true,
		LatencyMs:  50,
		Timestamp:  time.Now(),
	})

	state := latestState()
	require.Equal(t, string(types.HealthStateHealthy), state.HealthState)
	require.Equal(t, 0, state.ConsecutiveFailures)
	require.Empty(t, state.LastError)
	require.Equal(t, int64(50), state.LatencyMs)
}

func TestHealthChecker_UpdateHealthState_InferenceCheckClosesCircuitBreaker(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockCB := newMockCircuitBreaker()

	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1)

	checker := &healthCheckerImpl{
		circuitBreaker: mockCB,
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
				},
			},
		},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1,
		ModelName:  "gpt-4",
		Endpoint:   "https://api.example.com",
		CheckType:  types.HealthCheckTypeInference,
		Healthy:    true,
		LatencyMs:  50,
		Timestamp:  time.Now(),
	})

	require.Equal(t, string(types.HealthStateHealthy), latestState().HealthState)
	require.True(t, mockCB.forceClosed)
	require.Equal(t, int64(1), mockCB.forceClosedUpstreamID)
}

// mockCircuitBreaker implements CircuitBreaker for testing.
type mockCircuitBreaker struct {
	forceClosed           bool
	forceClosedUpstreamID int64
}

func newMockCircuitBreaker() *mockCircuitBreaker {
	return &mockCircuitBreaker{}
}

func (m *mockCircuitBreaker) Start(_ context.Context) error                        { return nil }
func (m *mockCircuitBreaker) Stop() error                                          { return nil }
func (m *mockCircuitBreaker) IsAvailable(_ context.Context, _ int64) (bool, error) { return true, nil }
func (m *mockCircuitBreaker) RecordSuccess(_ context.Context, _ int64) error       { return nil }
func (m *mockCircuitBreaker) RecordFailure(_ context.Context, _ int64, _ string, _ error) error {
	return nil
}
func (m *mockCircuitBreaker) GetCircuitState(_ context.Context, _ int64) (*types.ProviderCircuitStatus, error) {
	return nil, nil
}
func (m *mockCircuitBreaker) GetAllCircuitStates(_ context.Context) ([]types.ProviderCircuitStatus, error) {
	return nil, nil
}
func (m *mockCircuitBreaker) ForceOpen(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockCircuitBreaker) ForceClose(_ context.Context, upstreamID int64) error {
	m.forceClosed = true
	m.forceClosedUpstreamID = upstreamID
	return nil
}

func TestHealthChecker_GetHealthState_FromStore(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	now := time.Now()
	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(&database.AIGatewayUpstreamHealthState{
			UpstreamID:          1,
			HealthState:         string(types.HealthStateHealthy),
			LastCheckAt:         now,
			ConsecutiveFailures: 0,
			LatencyMs:           50,
		}, nil)

	checker := &healthCheckerImpl{
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	status, err := checker.GetHealthState(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, types.HealthStateHealthy, status.HealthState)
	require.Equal(t, int64(1), status.UpstreamID)
	require.Equal(t, int64(50), status.LatencyMs)
}

func TestHealthChecker_GetHealthState_StoreError(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(999)).
		Return(nil, errors.New("not found"))

	checker := &healthCheckerImpl{
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
	}

	status, err := checker.GetHealthState(context.Background(), 999)
	require.Error(t, err)
	require.Nil(t, status)
}

func TestHealthChecker_GetHealthState_FromCache(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	now := time.Now()
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().GetHealthState(mock.Anything, int64(1)).
		Return(&types.ProviderHealthStatus{
			UpstreamID:  1,
			HealthState: types.HealthStateHealthy,
			LastCheckAt: now,
			LatencyMs:   50,
		}, nil)

	checker := &healthCheckerImpl{
		healthStore: nil,
		stateCache:  cache,
	}

	status, err := checker.GetHealthState(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, types.HealthStateHealthy, status.HealthState)
}

func TestHealthChecker_UpdateLeadership_CacheDisabled(t *testing.T) {
	checker := &healthCheckerImpl{
		stateCache: NewStateCache(nil),
	}

	checker.isLeader.Store(false)
	checker.updateLeadership(context.Background())

	require.True(t, checker.isLeader.Load())
}

func TestHealthChecker_UpdateLeadership_AcquireSuccess(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().TryAcquireLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(true, nil)

	checker := &healthCheckerImpl{
		stateCache:   cache,
		leaderNodeID: "test-node-1",
	}

	checker.isLeader.Store(false)
	checker.updateLeadership(context.Background())

	require.True(t, checker.isLeader.Load())
}

func TestHealthChecker_UpdateLeadership_AcquireFails(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().RenewLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(false, nil)
	cache.EXPECT().TryAcquireLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(false, nil)
	cache.EXPECT().GetLeader(mock.Anything, "health-checker").
		Return("", errors.New("not found"))

	checker := &healthCheckerImpl{
		stateCache:   cache,
		leaderNodeID: "test-node-1",
	}

	checker.isLeader.Store(true)
	checker.updateLeadership(context.Background())

	require.False(t, checker.isLeader.Load())
}

func TestHealthChecker_UpdateLeadership_AcquireError(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().TryAcquireLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(false, errors.New("redis error"))

	checker := &healthCheckerImpl{
		stateCache:   cache,
		leaderNodeID: "test-node-1",
	}

	checker.isLeader.Store(false)
	checker.updateLeadership(context.Background())

	require.False(t, checker.isLeader.Load())
}

func TestHealthChecker_UpdateLeadership_RenewSuccess(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().RenewLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(true, nil)

	checker := &healthCheckerImpl{
		stateCache:   cache,
		leaderNodeID: "test-node-1",
	}

	checker.isLeader.Store(true)
	checker.updateLeadership(context.Background())

	require.True(t, checker.isLeader.Load())
}

func TestHealthChecker_UpdateLeadership_RenewFailsThenAcquire(t *testing.T) {
	cache := mockavailability.NewMockStateCache(t)
	cache.EXPECT().Enabled().Return(true)
	cache.EXPECT().RenewLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(false, nil)
	cache.EXPECT().TryAcquireLeader(mock.Anything, "health-checker", "test-node-1", 15*time.Second).
		Return(true, nil)

	checker := &healthCheckerImpl{
		stateCache:   cache,
		leaderNodeID: "test-node-1",
	}

	checker.isLeader.Store(true)
	checker.updateLeadership(context.Background())

	require.True(t, checker.isLeader.Load())
}

func TestHealthChecker_RunLeaderElection_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checker := &healthCheckerImpl{
		stateCache: NewStateCache(nil),
		stopCh:     make(chan struct{}),
	}

	checker.wg.Add(1)
	checker.runLeaderElection(ctx)
}

func TestHealthChecker_RunLeaderElection_ExitsOnStopCh(t *testing.T) {
	checker := &healthCheckerImpl{
		stateCache: NewStateCache(nil),
		stopCh:     make(chan struct{}),
	}
	close(checker.stopCh)

	checker.wg.Add(1)
	checker.runLeaderElection(context.Background())
}

func TestHealthChecker_RunL7APICheckRoutine_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checker := &healthCheckerImpl{
		stopCh: make(chan struct{}),
	}

	checker.wg.Add(1)
	checker.runL7APICheckRoutine(ctx)
}

func TestHealthChecker_RunL7APICheckRoutine_ExitsOnStopCh(t *testing.T) {
	checker := &healthCheckerImpl{
		stopCh: make(chan struct{}),
	}
	close(checker.stopCh)

	checker.wg.Add(1)
	checker.runL7APICheckRoutine(context.Background())
}

func TestHealthChecker_PerformL7APIChecks_NotLeader(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)

	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
	}
	checker.isLeader.Store(false)

	checker.performL7APIChecks(context.Background())
}

func TestHealthChecker_PerformL7APIChecks_StoreError(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)

	mockUpstreamStore.EXPECT().ListHealthCheckEnabled(mock.Anything).
		Return(nil, errors.New("db error"))

	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
	}
	checker.isLeader.Store(true)

	checker.performL7APIChecks(context.Background())
}

func TestHealthChecker_PerformL7APICheck_UrlNotChatCompletions(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
	}

	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completionsxxx",
		ModelName: "gpt-4",
		Provider:  "openai",
	})

	require.NotNil(t, result)
	require.True(t, result.Healthy)
	require.Equal(t, types.HealthCheckTypeL7API, result.CheckType)
	require.False(t, result.UsedInferenceFallback)
}

func TestHealthChecker_PerformL7APICheck_HTTPRequestSuccess(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					require.Equal(t, "GET", req.Method)
					require.Contains(t, req.URL.String(), "/models")

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{"data":[]}`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	})

	require.NotNil(t, result)
	require.True(t, result.Healthy)
	require.GreaterOrEqual(t, result.LatencyMs, int64(0))
	require.Equal(t, types.HealthCheckTypeL7API, result.CheckType)
}

func TestHealthChecker_PerformL7APICheck_HTTPRequestFailure(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable,
						Body:       io.NopCloser(bytes.NewReader([]byte(`error`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	})

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.Contains(t, result.Error, "503")
}

func TestHealthChecker_PerformL7APICheck_HTTPClientError(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					return nil, errors.New("connection refused")
				},
			},
		},
	}

	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	})

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.Contains(t, result.Error, "connection refused")
}

func TestHealthChecker_PerformL7APICheck_InvalidAuthHeader(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
	}

	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:         1,
		URL:        "https://api.example.com/v1/chat/completions",
		ModelName:  "gpt-4",
		Provider:   "openai",
		AuthHeader: `invalid json {`,
	})

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.NotEmpty(t, result.Error)
}

func TestHealthChecker_PerformInferenceCheck_Success(t *testing.T) {
	var capturedBody []byte
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					require.Equal(t, "POST", req.Method)
					require.Equal(t, "application/json", req.Header.Get("Content-Type"))

					var err error
					capturedBody, err = io.ReadAll(req.Body)
					require.NoError(t, err)

					var reqBody map[string]interface{}
					err = json.Unmarshal(capturedBody, &reqBody)
					require.NoError(t, err)
					require.Equal(t, "gpt-4", reqBody["model"])
					require.Equal(t, float64(1), reqBody["max_tokens"])
					require.NotEqual(t, true, reqBody["stream"])

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"test"}`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	}, 10*time.Second)

	require.NotNil(t, result)
	require.True(t, result.Healthy)
	require.GreaterOrEqual(t, result.LatencyMs, int64(0))
	require.Equal(t, types.HealthCheckTypeInference, result.CheckType)
}

func TestHealthChecker_PerformSampleCheck_UsesProviderExecution(t *testing.T) {
	called := false
	provider := executingSampleProvider{
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, client types.HTTPDoer) (*types.SampleExecutionResult, error) {
			called = true
			require.Equal(t, types.SampleKindInference, kind)
			require.Equal(t, int64(1024), input.MaxResponseBodyBytes)
			require.Nil(t, client)
			return &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Latency:    25 * time.Millisecond,
			}, nil
		},
	}
	checker := &healthCheckerImpl{
		sampleRegistry: sample.NewRegistry(provider),
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://example.com/custom",
		ModelName: "test-model",
	}, 10*time.Second)

	require.True(t, called)
	require.True(t, result.Healthy)
	require.Equal(t, int64(25), result.LatencyMs)
}

func TestHealthChecker_PerformSampleCheck_ResolvesTasksForInference(t *testing.T) {
	ctx := context.Background()
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmConfigStore.EXPECT().GetByID(ctx, int64(7)).Return(&database.LLMConfig{
		ID:       7,
		Metadata: map[string]any{"tasks": []any{"auto-speech-recognition"}},
	}, nil).Once()

	var capturedTasks []string
	provider := executingSampleProvider{
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			require.Equal(t, types.SampleKindInference, kind)
			capturedTasks = input.Tasks
			return &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Latency:    25 * time.Millisecond,
			}, nil
		},
	}
	checker := &healthCheckerImpl{
		sampleRegistry: sample.NewRegistry(provider),
		llmConfigStore: llmConfigStore,
	}

	result := checker.performInferenceCheck(ctx, &database.Upstream{
		ID:          1,
		URL:         "https://example.com/custom",
		ModelName:   "test-model",
		LLMConfigID: 7,
	}, 10*time.Second)

	require.True(t, result.Healthy)
	require.Equal(t, []string{"auto-speech-recognition"}, capturedTasks)
}

func TestHealthChecker_PerformSampleCheck_DoesNotResolveTasksForL7(t *testing.T) {
	ctx := context.Background()
	// No GetByID expectation: the L7 path must not touch the LLMConfigStore.
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)

	var capturedTasks []string
	provider := executingSampleProvider{
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			require.Equal(t, types.SampleKindL7API, kind)
			capturedTasks = input.Tasks
			return &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
			}, nil
		},
	}
	checker := &healthCheckerImpl{
		sampleRegistry: sample.NewRegistry(provider),
		llmConfigStore: llmConfigStore,
	}

	result := checker.performL7APICheck(ctx, &database.Upstream{
		ID:          1,
		URL:         "https://example.com/custom",
		ModelName:   "test-model",
		LLMConfigID: 7,
	})

	require.True(t, result.Healthy)
	require.Nil(t, capturedTasks)
}

func TestHealthChecker_PerformResponsesChecks_Success(t *testing.T) {
	requests := make([]*http.Request, 0, 2)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck: types.L7APICheckConfig{Timeout: 5 * time.Second},
		}},
		httpClient: &http.Client{Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"data":[]}`))),
					Header:     make(http.Header),
				}, nil
			},
		}},
	}
	upstream := &database.Upstream{
		ID:         1,
		URL:        "https://api.example.com/v1/responses",
		ModelName:  "responses-model",
		Provider:   "openai",
		AuthHeader: "Bearer test-token",
	}

	l7Result := checker.performL7APICheck(context.Background(), upstream)
	inferenceResult := checker.performInferenceCheck(context.Background(), upstream, 10*time.Second)

	require.True(t, l7Result.Healthy)
	require.True(t, inferenceResult.Healthy)
	require.Len(t, requests, 2)
	require.Equal(t, http.MethodGet, requests[0].Method)
	require.Equal(t, "https://api.example.com/v1/models", requests[0].URL.String())
	require.Equal(t, "Bearer test-token", requests[0].Header.Get("Authorization"))
	require.Equal(t, http.MethodPost, requests[1].Method)
	require.Equal(t, "https://api.example.com/v1/responses", requests[1].URL.String())
	require.Equal(t, "application/json", requests[1].Header.Get("Content-Type"))
	var body map[string]any
	require.NoError(t, json.NewDecoder(requests[1].Body).Decode(&body))
	require.Equal(t, "responses-model", body["model"])
	require.Equal(t, "hi", body["input"])
	// The generic Responses sample uses DashScope's minimum accepted output limit.
	require.Equal(t, float64(16), body["max_output_tokens"])
	require.NotEqual(t, true, body["stream"])
}

func TestHealthChecker_PerformUpstreamHealthCheck_FallbackUsesInferenceTimeout(t *testing.T) {
	const inferenceTimeout = 5 * time.Minute
	l7Timeout := time.Second
	requests := make([]*http.Request, 0, 2)
	deadlines := make([]time.Time, 0, 2)
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	expectStatefulMultimodalHealthStore(t, mockStore, 1)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck: types.L7APICheckConfig{Timeout: l7Timeout},
		}},
		healthStore: mockStore,
		stateCache:  NewStateCache(nil),
		httpClient: &http.Client{Transport: &mockTransport{roundTrip: func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req)
			deadline, ok := req.Context().Deadline()
			require.True(t, ok)
			deadlines = append(deadlines, deadline)
			if req.Method == http.MethodGet {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader([]byte("not found"))), Header: make(http.Header)}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"data":[]}`))), Header: make(http.Header)}, nil
		}}},
	}
	upstream := &database.Upstream{
		ID:         1,
		URL:        "https://api.example.com/v1/images/generations",
		ModelName:  "image-model",
		Provider:   "openai",
		AuthHeader: "Bearer test-token",
	}

	start := time.Now()
	checker.performUpstreamHealthCheck(context.Background(), upstream)

	require.Len(t, requests, 2)
	require.Len(t, deadlines, 2)
	require.Equal(t, "https://api.example.com/v1/models", requests[0].URL.String())
	require.Equal(t, upstream.URL, requests[1].URL.String())
	require.Equal(t, http.MethodGet, requests[0].Method)
	require.Equal(t, http.MethodPost, requests[1].Method)
	require.Equal(t, "Bearer test-token", requests[0].Header.Get("Authorization"))
	require.Equal(t, "Bearer test-token", requests[1].Header.Get("Authorization"))
	require.InDelta(t, l7Timeout, deadlines[0].Sub(start), float64(500*time.Millisecond))
	require.InDelta(t, inferenceTimeout, deadlines[1].Sub(start), float64(500*time.Millisecond))
}

func TestHealthChecker_PerformMessagesChecks_Success(t *testing.T) {
	requests := make([]*http.Request, 0, 2)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck: types.L7APICheckConfig{Timeout: 5 * time.Second},
		}},
		httpClient: &http.Client{Transport: &mockTransport{roundTrip: func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"content":[]}`))), Header: make(http.Header)}, nil
		}}},
	}
	upstream := &database.Upstream{URL: "https://api.example.com/v1/messages", ModelName: "claude-model", AuthHeader: `{"x-api-key":"secret"}`}

	require.True(t, checker.performL7APICheck(context.Background(), upstream).Healthy)
	require.True(t, checker.performInferenceCheck(context.Background(), upstream, 10*time.Second).Healthy)
	require.Len(t, requests, 2)
	require.Equal(t, http.MethodGet, requests[0].Method)
	require.Equal(t, "https://api.example.com/v1/models", requests[0].URL.String())
	require.Equal(t, "secret", requests[0].Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", requests[0].Header.Get("anthropic-version"))
	require.Equal(t, http.MethodPost, requests[1].Method)
	require.Equal(t, upstream.URL, requests[1].URL.String())
	require.Equal(t, "secret", requests[1].Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", requests[1].Header.Get("anthropic-version"))
}

func TestHealthChecker_PerformResponsesInferenceCheck_Failure(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck: types.L7APICheckConfig{Timeout: 5 * time.Second},
		}},
		httpClient: &http.Client{Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://api.example.com/v1/responses", req.URL.String())
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(bytes.NewReader([]byte(`responses unavailable`))),
					Header:     make(http.Header),
				}, nil
			},
		}},
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/responses",
		ModelName: "responses-model",
	}, 10*time.Second)

	require.False(t, result.Healthy)
	require.Contains(t, result.Error, "503")
	require.Contains(t, result.Error, "responses unavailable")
}

func TestHealthChecker_PerformL7APIChecks_SkipsUnsupportedProtocol(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)
	var httpCallCount atomic.Int32
	mockUpstreamStore.EXPECT().ListHealthCheckEnabled(mock.Anything).Return([]*database.Upstream{{
		ID:  1,
		URL: "https://api.example.com/v1/ocr",
	}}, nil).Once()
	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
		httpClient: &http.Client{Transport: &mockTransport{roundTrip: func(_ *http.Request) (*http.Response, error) {
			httpCallCount.Add(1)
			return nil, nil
		}}},
	}
	checker.isLeader.Store(true)

	checker.performL7APIChecks(context.Background())
	require.Zero(t, httpCallCount.Load())
}

func TestHealthChecker_PerformInferenceCheck_Failure(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Body:       io.NopCloser(bytes.NewReader([]byte(`rate limited`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	}, 10*time.Second)

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.Contains(t, result.Error, "429")
}

func TestHealthChecker_PerformInferenceCheck_HTTPClientError(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					return nil, errors.New("timeout")
				},
			},
		},
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	}, 10*time.Second)

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.Contains(t, result.Error, "timeout")
}

func TestHealthChecker_PerformInferenceCheck_InvalidAuthHeader(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
	}

	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:         1,
		URL:        "https://api.example.com/v1/chat/completions",
		ModelName:  "gpt-4",
		Provider:   "openai",
		AuthHeader: `invalid json {`,
	}, 10*time.Second)

	require.NotNil(t, result)
	require.False(t, result.Healthy)
	require.NotEmpty(t, result.Error)
}

func TestHealthChecker_PerformL7APIChecks_LeaderWithUpstreams(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)

	upstreams := []*database.Upstream{
		{
			ID:        1,
			URL:       "https://api1.example.com/v1/chat/completions",
			ModelName: "gpt-4",
			Provider:  "openai",
		},
		{
			ID:        2,
			URL:       "https://api2.example.com/v1/chat/completions",
			ModelName: "claude-3",
			Provider:  "anthropic",
		},
	}

	mockUpstreamStore.EXPECT().ListHealthCheckEnabled(mock.Anything).
		Return(upstreams, nil)

	var upsertCount atomic.Int32
	mockStore.EXPECT().MutateByUpstreamID(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, mutation database.AIGatewayUpstreamHealthStateMutation) (*database.AIGatewayUpstreamHealthState, error) {
			state := &database.AIGatewayUpstreamHealthState{UpstreamID: mutation.UpstreamID, HealthState: string(types.HealthStateHealthy)}
			require.NoError(t, mutation.Mutate(state))
			upsertCount.Add(1)
			return state, nil
		}).
		Times(2)

	httpCallCount := atomic.Int32{}
	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
		stateCache:    NewStateCache(nil),
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
				HealthRules: types.HealthRulesConfig{
					ConsecutiveFailuresForUnhealthy: 3,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					httpCallCount.Add(1)
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{"data":[]}`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}
	checker.isLeader.Store(true)

	checker.performL7APIChecks(context.Background())

	require.Equal(t, int32(4), httpCallCount.Load(),
		"should make HTTP calls for each upstream (L7 API + inference)")
	require.Equal(t, int32(2), upsertCount.Load(),
		"should upsert health state for each upstream")
}

func TestHealthChecker_PerformInferenceCheck_UsesProvidedTimeout(t *testing.T) {
	expectedTimeout := 2 * time.Minute
	var capturedDeadline time.Time

	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: time.Second,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					deadline, ok := req.Context().Deadline()
					require.True(t, ok, "request context should have a deadline")
					capturedDeadline = deadline
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	start := time.Now()
	result := checker.performInferenceCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	}, expectedTimeout)

	require.NotNil(t, result)
	require.True(t, result.Healthy)

	actualDuration := capturedDeadline.Sub(start)
	require.InDelta(t, expectedTimeout, actualDuration, float64(500*time.Millisecond),
		"inference check should use the provided timeout (%v) as context deadline, got %v", expectedTimeout, actualDuration)
}

func TestHealthChecker_PerformUpstreamHealthCheck_UsesInferencePolicy(t *testing.T) {
	expectedTimeout := 2 * time.Minute
	inferenceInterval := time.Hour
	probeStartedAt := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	probeCompletedAt := probeStartedAt.Add(40 * time.Second)
	var nowCalls atomic.Int32
	var capturedDeadline time.Time
	provider := executingSampleProvider{
		executionPolicy: func(kind types.SampleKind) (types.SampleExecutionPolicy, error) {
			require.Equal(t, types.SampleKindInference, kind)
			return types.SampleExecutionPolicy{Timeout: expectedTimeout, Multimodal: true}, nil
		},
		execute: func(ctx context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			latency := time.Duration(0)
			if kind == types.SampleKindInference {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				capturedDeadline = deadline
				latency = 40733 * time.Millisecond
			}
			return &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Latency:    latency,
			}, nil
		},
	}
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 2)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Timeout: time.Second},
			MultimodalInferenceInterval: inferenceInterval,
			HealthRules: types.HealthRulesConfig{
				ConsecutiveFailuresForUnhealthy:       3,
				LatencyThresholdForDegraded:           10 * time.Second,
				MultimodalLatencyThresholdForDegraded: 2 * time.Minute,
			},
		}},
		healthStore:    mockStore,
		stateCache:     NewStateCache(nil),
		sampleRegistry: sample.NewRegistry(provider),
		now: func() time.Time {
			if nowCalls.Add(1) == 1 {
				return probeStartedAt
			}
			return probeCompletedAt
		},
	}
	upstream := &database.Upstream{ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model"}

	start := time.Now()
	checker.performUpstreamHealthCheck(context.Background(), upstream)

	require.False(t, capturedDeadline.IsZero())
	require.InDelta(t, expectedTimeout, capturedDeadline.Sub(start), float64(500*time.Millisecond))
	require.NotNil(t, latestState())
	require.Equal(t, string(types.HealthStateHealthy), latestState().HealthState)
	probeState, ok := checker.multimodalProbes.snapshot(upstream.ID)
	require.True(t, ok)
	require.Equal(t, probeCompletedAt.Add(inferenceInterval), probeState.nextInferenceAt)
}

func TestHealthChecker_PerformUpstreamHealthCheck_UsesMultimodalThresholdForInferenceFallback(t *testing.T) {
	provider := executingSampleProvider{
		executionPolicy: func(kind types.SampleKind) (types.SampleExecutionPolicy, error) {
			require.Equal(t, types.SampleKindInference, kind)
			return types.SampleExecutionPolicy{Timeout: 5 * time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			result := &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
			}
			if kind == types.SampleKindL7API {
				result.InferenceFallbackRequired = true
				return result, nil
			}
			result.Latency = 40733 * time.Millisecond
			return result, nil
		},
	}
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	latestState := expectStatefulMultimodalHealthStore(t, mockStore, 1)

	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck: types.L7APICheckConfig{Timeout: time.Second},
			HealthRules: types.HealthRulesConfig{
				ConsecutiveFailuresForUnhealthy:       3,
				LatencyThresholdForDegraded:           10 * time.Second,
				MultimodalLatencyThresholdForDegraded: 2 * time.Minute,
			},
		}},
		healthStore:    mockStore,
		stateCache:     NewStateCache(nil),
		sampleRegistry: sample.NewRegistry(provider),
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/images/generations",
		ModelName: "image-model",
	})

	require.Equal(t, string(types.HealthStateHealthy), latestState().HealthState)
}

func TestHealthChecker_PerformUpstreamHealthCheck_RejectsInvalidInferenceTimeout(t *testing.T) {
	executed := false
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{}, nil
		},
		execute: func(context.Context, types.SampleKind, types.SampleInput, types.HTTPDoer) (*types.SampleExecutionResult, error) {
			executed = true
			return nil, nil
		},
	}
	checker := &healthCheckerImpl{sampleRegistry: sample.NewRegistry(provider)}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{ID: 1, URL: "https://api.example.com/v1/images/generations"})

	require.False(t, executed)
}

func TestHealthChecker_PerformL7APICheck_UsesSingleTimeout(t *testing.T) {
	baseTimeout := 3 * time.Second
	var capturedDeadline time.Time

	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: baseTimeout,
				},
			},
		},
		httpClient: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					deadline, ok := req.Context().Deadline()
					require.True(t, ok, "request context should have a deadline")
					capturedDeadline = deadline
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{"data":[]}`))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	start := time.Now()
	result := checker.performL7APICheck(context.Background(), &database.Upstream{
		ID:        1,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "gpt-4",
		Provider:  "openai",
	})

	require.NotNil(t, result)
	require.True(t, result.Healthy)

	// The context deadline should be approximately baseTimeout from start
	actualDuration := capturedDeadline.Sub(start)
	require.InDelta(t, baseTimeout, actualDuration, float64(500*time.Millisecond),
		"L7 API check should use Timeout (%v) as context deadline, got %v", baseTimeout, actualDuration)
}

func TestHealthChecker_HttpClientHasNoClientLevelTimeout(t *testing.T) {
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{
			Config: types.HealthCheckConfig{
				L7APICheck: types.L7APICheckConfig{
					Timeout: 5 * time.Second,
				},
			},
		},
		httpClient: &http.Client{},
	}

	require.Zero(t, checker.httpClient.Timeout,
		"httpClient should have no client-level timeout; per-request context.WithTimeout controls timeouts")
}

func TestNewHealthChecker_HealthCheckIntervalDefaultsAndOverrides(t *testing.T) {
	tests := []struct {
		name              string
		l7IntervalSeconds int
		l7TimeoutSeconds  int
		modalSeconds      int
		expectedL7        time.Duration
		expectedTimeout   time.Duration
		expectedModal     time.Duration
	}{
		{
			name:            "non-positive values use defaults",
			expectedL7:      defaultHealthCheckL7Interval,
			expectedTimeout: defaultHealthCheckL7Timeout,
			expectedModal:   defaultMultimodalInferenceCheckInterval,
		},
		{
			name:              "configured values are preserved",
			l7IntervalSeconds: 90,
			l7TimeoutSeconds:  20,
			modalSeconds:      7200,
			expectedL7:        90 * time.Second,
			expectedTimeout:   20 * time.Second,
			expectedModal:     2 * time.Hour,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.AIGateway.HealthCheckL7APIInterval = test.l7IntervalSeconds
			cfg.AIGateway.HealthCheckL7APITimeout = test.l7TimeoutSeconds
			cfg.AIGateway.HealthCheckModalInferenceInterval = test.modalSeconds
			checker := NewHealthChecker(nil, cfg, nil, nil, nil, nil).(*healthCheckerImpl)

			require.Equal(t, test.expectedL7, checker.config.Config.L7APICheck.Interval)
			require.Equal(t, test.expectedTimeout, checker.config.Config.L7APICheck.Timeout)
			require.Equal(t, test.expectedModal, checker.config.Config.MultimodalInferenceInterval)
		})
	}
}

func TestNewHealthChecker_MapsLatencyThresholds(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.HealthCheckLatencyDegradedMs = 10000
	cfg.AIGateway.HealthCheckMultimodalLatencyDegradedMs = 120000

	checker := NewHealthChecker(nil, cfg, nil, nil, nil, nil).(*healthCheckerImpl)

	require.Equal(t, 10*time.Second, checker.config.Config.HealthRules.LatencyThresholdForDegraded)
	require.Equal(t, 2*time.Minute, checker.config.Config.HealthRules.MultimodalLatencyThresholdForDegraded)
}

func TestBootstrapMultimodalProbeStateUsesPersistedInferenceFacts(t *testing.T) {
	checkedAt := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	policy := multimodalProbePolicy{
		retryInterval: time.Minute, inferenceInterval: time.Hour, failureThreshold: 3,
	}

	for _, test := range []struct {
		name             string
		healthState      *database.AIGatewayUpstreamHealthState
		expectedDeadline time.Time
	}{
		{name: "new upstream is immediately due"},
		{name: "legacy row is immediately due", healthState: &database.AIGatewayUpstreamHealthState{
			Metadata: map[string]any{"legacy": true},
		}},
		{name: "successful inference uses normal interval", healthState: multimodalDatabaseStateForTest(
			probeHealthState{LastCheckedAt: checkedAt},
		), expectedDeadline: checkedAt.Add(time.Hour)},
		{name: "pending inference failure uses retry interval", healthState: multimodalDatabaseStateForTest(
			probeHealthState{ConsecutiveFailures: 1, LastCheckedAt: checkedAt},
		), expectedDeadline: checkedAt.Add(time.Minute)},
		{name: "threshold failure uses normal interval", healthState: multimodalDatabaseStateForTest(
			probeHealthState{ConsecutiveFailures: 3, LastCheckedAt: checkedAt},
		), expectedDeadline: checkedAt.Add(time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, err := bootstrapMultimodalProbeState(test.healthState, policy)
			require.NoError(t, err)
			require.Equal(t, test.expectedDeadline, state.nextInferenceAt)
			require.Equal(t, multimodalProbeL7Available, state.mode)
		})
	}
}

func multimodalDatabaseStateForTest(inference probeHealthState) *database.AIGatewayUpstreamHealthState {
	state := &database.AIGatewayUpstreamHealthState{}
	writeMultimodalHealthState(state, multimodalHealthState{Inference: inference})
	return state
}

func TestHealthChecker_PerformL7APIChecks_ThrottlesMultimodalInference(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstream := &database.Upstream{ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model"}
	mockUpstreamStore.EXPECT().ListHealthCheckEnabled(mock.Anything).Return([]*database.Upstream{upstream}, nil).Times(2)
	expectStatefulMultimodalHealthStore(t, mockStore, 3)

	var calls atomic.Int32
	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
		stateCache:    NewStateCache(nil),
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 5 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		httpClient: &http.Client{Transport: &mockTransport{roundTrip: func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{}`))), Header: make(http.Header)}, nil
		}}},
		now: func() time.Time { return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC) },
	}
	checker.isLeader.Store(true)

	checker.performL7APIChecks(context.Background())
	checker.performL7APIChecks(context.Background())

	require.Equal(t, int32(3), calls.Load(), "first tick runs L7 and inference; second tick runs only L7")
}

func TestHealthChecker_PerformL7APIChecks_ThrottlesInferenceFallback(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockUpstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstream := &database.Upstream{ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model"}
	mockUpstreamStore.EXPECT().ListHealthCheckEnabled(mock.Anything).Return([]*database.Upstream{upstream}, nil).Times(4)
	expectStatefulMultimodalHealthStore(t, mockStore, 4)

	var calls atomic.Int32
	var modelsAvailable atomic.Bool
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	var nowUnix atomic.Int64
	nowUnix.Store(start.Unix())
	checker := &healthCheckerImpl{
		healthStore:   mockStore,
		upstreamStore: mockUpstreamStore,
		stateCache:    NewStateCache(nil),
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 5 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		httpClient: &http.Client{Transport: &mockTransport{roundTrip: func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			if req.Method == http.MethodGet && !modelsAvailable.Load() {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader([]byte("not found"))), Header: make(http.Header)}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{}`))), Header: make(http.Header)}, nil
		}}},
		now: func() time.Time { return time.Unix(nowUnix.Load(), 0) },
	}
	checker.isLeader.Store(true)

	checker.performL7APIChecks(context.Background())
	checker.performL7APIChecks(context.Background())
	state, ok := checker.multimodalProbes.snapshot(1)
	require.True(t, ok)
	require.Equal(t, multimodalProbeInferenceOnly, state.mode)
	require.Equal(t, int32(2), calls.Load(), "fallback is the inference attempt and the next tick skips the whole upstream")

	modelsAvailable.Store(true)
	nowUnix.Store(start.Add(time.Hour).Unix())
	checker.performL7APIChecks(context.Background())
	state, ok = checker.multimodalProbes.snapshot(1)
	require.True(t, ok)
	require.Equal(t, multimodalProbeL7Available, state.mode)

	nowUnix.Store(start.Add(time.Hour + time.Minute).Unix())
	checker.performL7APIChecks(context.Background())
	require.Equal(t, int32(5), calls.Load(), "working models endpoint restores per-tick cheap L7 checks")
}

func TestHealthChecker_PerformUpstreamHealthCheck_UsesSharedDeadlineAfterFailover(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
		1: {mode: multimodalProbeL7Available, nextInferenceAt: now.Add(time.Hour)},
	}}
	var l7Calls atomic.Int32
	var inferenceCalls atomic.Int32
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: 5 * time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			if kind == types.SampleKindL7API {
				l7Calls.Add(1)
			} else {
				inferenceCalls.Add(1)
			}
			return &types.SampleExecutionResult{
				Request:    &types.SampleRequest{Endpoint: input.Endpoint},
				StatusCode: http.StatusOK,
				Status:     "200 OK",
			}, nil
		},
	}
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	expectStatefulMultimodalHealthStore(t, mockStore, 1)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		stateCache:       NewStateCache(nil),
		healthStore:      mockStore,
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
		now:              func() time.Time { return now },
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	require.Equal(t, int32(1), l7Calls.Load())
	require.Zero(t, inferenceCalls.Load())
	require.Equal(t, 1, shared.getCalls)
}

func TestHealthChecker_PerformUpstreamHealthCheck_StaleL7DoesNotAdvanceMultimodalCadence(t *testing.T) {
	now := time.Now()
	sharedState := multimodalProbeState{mode: multimodalProbeL7Available, nextInferenceAt: now}
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{1: sharedState}}
	var l7Calls atomic.Int32
	var inferenceCalls atomic.Int32
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			if kind == types.SampleKindL7API {
				l7Calls.Add(1)
			} else {
				inferenceCalls.Add(1)
			}
			return &types.SampleExecutionResult{
				Request: &types.SampleRequest{Endpoint: input.Endpoint}, StatusCode: http.StatusOK, Status: "200 OK",
			}, nil
		},
	}
	persisted := &database.AIGatewayUpstreamHealthState{UpstreamID: 1, HealthState: string(types.HealthStateHealthy)}
	writeMultimodalHealthState(persisted, multimodalHealthState{
		L7:        probeHealthState{LastCheckedAt: now.Add(time.Hour), LastSucceededAt: now.Add(time.Hour)},
		Inference: probeHealthState{LastCheckedAt: now.Add(-time.Hour), LastSucceededAt: now.Add(-time.Hour)},
	})
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	expectStatefulMultimodalHealthStore(t, store, 1, persisted)
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore:      store,
		stateCache:       NewStateCache(nil),
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
		now:              func() time.Time { return now },
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	require.Equal(t, int32(1), l7Calls.Load())
	require.Zero(t, inferenceCalls.Load())
	require.Equal(t, sharedState, shared.states[int64(1)])
}

func TestHealthChecker_MultimodalDatabaseFailureDoesNotAdvanceCadence(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
		1: {mode: multimodalProbeL7Available, nextInferenceAt: now},
	}}
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	store.EXPECT().MutateByUpstreamID(mock.Anything, mock.Anything).Return(nil, errTestSentinel).Once()
	var inferenceCalls atomic.Int32
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			if kind == types.SampleKindInference {
				inferenceCalls.Add(1)
			}
			return &types.SampleExecutionResult{
				Request: &types.SampleRequest{Endpoint: input.Endpoint}, StatusCode: http.StatusOK, Status: "200 OK",
			}, nil
		},
	}
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore:      store,
		stateCache:       NewStateCache(nil),
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
		now:              func() time.Time { return now },
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	require.Zero(t, inferenceCalls.Load())
	require.Equal(t, now, shared.states[int64(1)].nextInferenceAt)
}

func TestHealthChecker_MultimodalRedisFailureDoesNotDiscardPersistedL7Success(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{
		states: map[int64]multimodalProbeState{
			1: {mode: multimodalProbeInferenceOnly, nextInferenceAt: now},
		},
		setErr: errTestSentinel,
	}
	store := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	latestState := expectStatefulMultimodalHealthStore(t, store, 1)
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, _ types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			return &types.SampleExecutionResult{
				Request: &types.SampleRequest{Endpoint: input.Endpoint}, StatusCode: http.StatusOK, Status: "200 OK",
			}, nil
		},
	}
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		healthStore:      store,
		stateCache:       NewStateCache(nil),
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
		now:              func() time.Time { return now },
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	health, found, err := readMultimodalHealthState(latestState().Metadata)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, health.L7.LastSucceededAt.IsZero())
	require.Equal(t, string(types.HealthStateUnknown), latestState().HealthState,
		"a legacy row must not recover before the first inference fact is persisted")
	local, found := checker.multimodalProbes.snapshot(1)
	require.True(t, found)
	require.Equal(t, multimodalProbeL7Available, local.mode)
	require.Equal(t, multimodalProbeInferenceOnly, shared.states[int64(1)].mode)
}

func TestHealthChecker_PerformUpstreamHealthCheck_SkipsMultimodalProbeWhenRedisLoadFails(t *testing.T) {
	shared := &fakeMultimodalProbeStateCache{getErr: errTestSentinel}
	var executions atomic.Int32
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: 5 * time.Minute, Multimodal: true}, nil
		},
		execute: func(context.Context, types.SampleKind, types.SampleInput, types.HTTPDoer) (*types.SampleExecutionResult, error) {
			executions.Add(1)
			return nil, nil
		},
	}
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		stateCache:       NewStateCache(nil),
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	require.Zero(t, executions.Load())
}

func TestHealthChecker_PerformUpstreamHealthCheck_DoesNotFallbackBeforeSharedDeadline(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
		1: {mode: multimodalProbeL7Available, nextInferenceAt: now.Add(time.Hour)},
	}}
	var inferenceCalls atomic.Int32
	provider := executingSampleProvider{
		executionPolicy: func(types.SampleKind) (types.SampleExecutionPolicy, error) {
			return types.SampleExecutionPolicy{Timeout: 5 * time.Minute, Multimodal: true}, nil
		},
		execute: func(_ context.Context, kind types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
			result := &types.SampleExecutionResult{Request: &types.SampleRequest{Endpoint: input.Endpoint}}
			if kind == types.SampleKindL7API {
				result.InferenceFallbackRequired = true
				return result, nil
			}
			inferenceCalls.Add(1)
			result.StatusCode = http.StatusOK
			return result, nil
		},
	}
	checker := &healthCheckerImpl{
		config: HealthCheckerConfig{Config: types.HealthCheckConfig{
			L7APICheck:                  types.L7APICheckConfig{Interval: time.Minute, Timeout: 15 * time.Second},
			MultimodalInferenceInterval: time.Hour,
			HealthRules:                 types.HealthRulesConfig{ConsecutiveFailuresForUnhealthy: 3},
		}},
		stateCache:       NewStateCache(nil),
		sampleRegistry:   sample.NewRegistry(provider),
		multimodalProbes: multimodalProbeScheduler{stateCache: shared},
		now:              func() time.Time { return now },
	}

	checker.performUpstreamHealthCheck(context.Background(), &database.Upstream{
		ID: 1, URL: "https://api.example.com/v1/images/generations", ModelName: "image-model",
	})

	require.Zero(t, inferenceCalls.Load())
	require.Equal(t, multimodalProbeInferenceOnly, shared.states[int64(1)].mode)
	require.Equal(t, now.Add(time.Hour), shared.states[int64(1)].nextInferenceAt)
}

func TestHealthChecker_SetLeadershipClearsLocalProbeState(t *testing.T) {
	checker := &healthCheckerImpl{}
	checker.multimodalProbes.setLocal(1, multimodalProbeState{nextInferenceAt: time.Now().Add(time.Hour)})
	checker.isLeader.Store(true)

	checker.setLeadership(false)

	_, ok := checker.multimodalProbes.snapshot(1)
	require.False(t, ok)
}
