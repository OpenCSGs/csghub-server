package availability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	mockavailability "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component/availability"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

// mockTransport provides a configurable RoundTripper for HTTP tests.
type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestHealthChecker_UpdateHealthState_Degraded(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(nil, errors.New("not found"))

	var upsertedState *database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, state *database.AIGatewayUpstreamHealthState) {
			copied := *state
			upsertedState = &copied
		}).
		Return(nil)

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

	require.NotNil(t, upsertedState)
	require.Equal(t, string(types.HealthStateDegraded), upsertedState.HealthState)
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

	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(nil, errors.New("not found"))

	var upsertedState *database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, state *database.AIGatewayUpstreamHealthState) {
			copied := *state
			upsertedState = &copied
		}).
		Return(nil)

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

	require.NotNil(t, upsertedState)
	require.Equal(t, string(types.HealthStateHealthy), upsertedState.HealthState)
	require.Equal(t, int64(1), upsertedState.UpstreamID)
	require.Equal(t, int64(50), upsertedState.LatencyMs)
	require.Equal(t, 0, upsertedState.ConsecutiveFailures)
	require.Empty(t, upsertedState.LastError)
}

func TestHealthChecker_UpdateHealthState_NewUpstreamUnhealthy(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)

	var storedState *database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		RunAndReturn(func(_ context.Context, _ int64) (*database.AIGatewayUpstreamHealthState, error) {
			if storedState != nil {
				copied := *storedState
				return &copied, nil
			}
			return nil, errors.New("not found")
		}).
		Maybe()

	var upsertedStates []*database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, state *database.AIGatewayUpstreamHealthState) {
			copied := *state
			storedState = &copied
			upsertedStates = append(upsertedStates, &copied)
		}).
		Return(nil).
		Maybe()

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

	require.Len(t, upsertedStates, 3)
	finalState := upsertedStates[2]
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
	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(existingState, nil)

	var upsertedState *database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, state *database.AIGatewayUpstreamHealthState) {
			copied := *state
			upsertedState = &copied
		}).
		Return(nil)

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

	require.NotNil(t, upsertedState)
	require.Equal(t, string(types.HealthStateHealthy), upsertedState.HealthState)
	require.Equal(t, 0, upsertedState.ConsecutiveFailures)
	require.Empty(t, upsertedState.LastError)
	require.Equal(t, int64(50), upsertedState.LatencyMs)
}

func TestHealthChecker_UpdateHealthState_InferenceCheckClosesCircuitBreaker(t *testing.T) {
	mockStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	mockCB := newMockCircuitBreaker()

	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(nil, errors.New("not found"))

	var upsertedState *database.AIGatewayUpstreamHealthState
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, state *database.AIGatewayUpstreamHealthState) {
			copied := *state
			upsertedState = &copied
		}).
		Return(nil)

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

	require.NotNil(t, upsertedState)
	require.Equal(t, string(types.HealthStateHealthy), upsertedState.HealthState)
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

func (m *mockCircuitBreaker) Start(_ context.Context) error                     { return nil }
func (m *mockCircuitBreaker) Stop() error                                       { return nil }
func (m *mockCircuitBreaker) IsAvailable(_ context.Context, _ int64) (bool, error) { return true, nil }
func (m *mockCircuitBreaker) RecordSuccess(_ context.Context, _ int64) error    { return nil }
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
					require.Equal(t, false, reqBody["stream"])

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
	})

	require.NotNil(t, result)
	require.True(t, result.Healthy)
	require.GreaterOrEqual(t, result.LatencyMs, int64(0))
	require.Equal(t, types.HealthCheckTypeInference, result.CheckType)
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
	})

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
	})

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
	})

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

	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(1)).
		Return(nil, errors.New("not found"))
	mockStore.EXPECT().GetByUpstreamID(mock.Anything, int64(2)).
		Return(nil, errors.New("not found"))

	var upsertCount atomic.Int32
	mockStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *database.AIGatewayUpstreamHealthState) {
			upsertCount.Add(1)
		}).
		Return(nil).
		Maybe()

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

func TestHealthChecker_PerformInferenceCheck_UsesDoubleTimeout(t *testing.T) {
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
	})

	require.NotNil(t, result)
	require.True(t, result.Healthy)

	// The context deadline should be approximately baseTimeout * 2 from start
	expectedDuration := baseTimeout * 2
	actualDuration := capturedDeadline.Sub(start)
	require.InDelta(t, expectedDuration, actualDuration, float64(500*time.Millisecond),
		"inference check should use Timeout*2 (%v) as context deadline, got %v", expectedDuration, actualDuration)
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
