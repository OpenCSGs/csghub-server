package availability

import (
	"strconv"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
)

type fakeHealthStore struct {
	mu     sync.Mutex
	states map[string]*database.AIGatewayUpstreamHealthState
}

func newFakeHealthStore() *fakeHealthStore {
	return &fakeHealthStore{states: map[string]*database.AIGatewayUpstreamHealthState{}}
}

func (s *fakeHealthStore) key(upstreamID int64) string {
	return strconv.FormatInt(upstreamID, 10)
}

func (s *fakeHealthStore) Create(ctx context.Context, state *database.AIGatewayUpstreamHealthState) error {
	return s.Upsert(ctx, state)
}
func (s *fakeHealthStore) Update(ctx context.Context, state *database.AIGatewayUpstreamHealthState) error {
	return s.Upsert(ctx, state)
}

func (s *fakeHealthStore) Upsert(_ context.Context, state *database.AIGatewayUpstreamHealthState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := *state
	s.states[s.key(state.UpstreamID)] = &copied
	return nil
}

func (s *fakeHealthStore) GetByUpstreamID(_ context.Context, upstreamID int64) (*database.AIGatewayUpstreamHealthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.states[s.key(upstreamID)]; ok {
		copied := *state
		return &copied, nil
	}
	return nil, errors.New("not found")
}

func (s *fakeHealthStore) GetAllHealthy(_ context.Context) ([]database.AIGatewayUpstreamHealthState, error) {
	return nil, nil
}
func (s *fakeHealthStore) GetAllUnhealthy(_ context.Context) ([]database.AIGatewayUpstreamHealthState, error) {
	return nil, nil
}


func (s *fakeHealthStore) DeleteByUpstreamID(_ context.Context, upstreamID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, s.key(upstreamID))
	return nil
}

func TestHealthChecker_UpdateHealthState_Degraded(t *testing.T) {
	store := newFakeHealthStore()
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
		healthStore: store,
		stateCache:  NewStateCache(nil),
	}

	checker.updateHealthState(context.Background(), &types.HealthCheckResult{
		UpstreamID: 1,
		ModelName: "gpt-4",
		Endpoint:  "https://api.example.com",
		Healthy:   true,
		LatencyMs: 3000,
		Timestamp: time.Now(),
	})

	state, err := store.GetByUpstreamID(context.Background(), int64(1))
	require.NoError(t, err)
	require.Equal(t, string(types.HealthStateDegraded), state.HealthState)
}
