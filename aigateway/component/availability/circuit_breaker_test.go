package availability

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/types"
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
