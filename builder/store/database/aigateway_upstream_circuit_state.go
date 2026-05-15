package database

import (
	"context"
	"fmt"
	"time"
)

// AIGatewayUpstreamCircuitState tracks circuit breaker state per upstream.
type AIGatewayUpstreamCircuitState struct {
	ID              int64          `bun:",pk,autoincrement" json:"id"`
	UpstreamID      int64          `bun:",unique,notnull" json:"upstream_id"`
	Upstream        *Upstream      `bun:"rel:belongs-to,join:upstream_id=id" json:"-"`
	CircuitState    string         `bun:",notnull,default:closed" json:"circuit_state"` // closed, open, half_open
	FailureCount    int            `bun:",notnull,default:0" json:"failure_count"`
	SuccessCount    int            `bun:",notnull,default:0" json:"success_count"`
	LastStateChange time.Time      `bun:",notnull" json:"last_state_change"`
	NextRetryAt     *time.Time     `bun:",nullzero" json:"next_retry_at"`
	Metadata        map[string]any `bun:",type:jsonb,nullzero" json:"metadata"`
	times
}

type AIGatewayUpstreamCircuitStateStore interface {
	Create(ctx context.Context, state *AIGatewayUpstreamCircuitState) error
	Update(ctx context.Context, state *AIGatewayUpstreamCircuitState) error
	Upsert(ctx context.Context, state *AIGatewayUpstreamCircuitState) error
	GetByUpstreamID(ctx context.Context, upstreamID int64) (*AIGatewayUpstreamCircuitState, error)
	GetAllOpen(ctx context.Context) ([]AIGatewayUpstreamCircuitState, error)
	GetAllClosed(ctx context.Context) ([]AIGatewayUpstreamCircuitState, error)
	DeleteByUpstreamID(ctx context.Context, upstreamID int64) error
}

type aigatewayUpstreamCircuitStateStoreImpl struct {
	db *DB
}

func NewAIGatewayUpstreamCircuitStateStore() AIGatewayUpstreamCircuitStateStore {
	return &aigatewayUpstreamCircuitStateStoreImpl{db: defaultDB}
}

func NewAIGatewayUpstreamCircuitStateStoreWithDB(db *DB) AIGatewayUpstreamCircuitStateStore {
	return &aigatewayUpstreamCircuitStateStoreImpl{db: db}
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) Create(ctx context.Context, state *AIGatewayUpstreamCircuitState) error {
	_, err := s.db.Core.NewInsert().Model(state).Exec(ctx)
	return err
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) Update(ctx context.Context, state *AIGatewayUpstreamCircuitState) error {
	_, err := s.db.Core.NewUpdate().Model(state).Where("id = ?", state.ID).Exec(ctx)
	return err
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) Upsert(ctx context.Context, state *AIGatewayUpstreamCircuitState) error {
	_, err := s.db.Core.NewInsert().
		Model(state).
		On("CONFLICT (upstream_id) DO UPDATE").
		Set("circuit_state = EXCLUDED.circuit_state").
		Set("failure_count = EXCLUDED.failure_count").
		Set("success_count = EXCLUDED.success_count").
		Set("last_state_change = EXCLUDED.last_state_change").
		Set("next_retry_at = EXCLUDED.next_retry_at").
		Set("metadata = EXCLUDED.metadata").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) GetByUpstreamID(ctx context.Context, upstreamID int64) (*AIGatewayUpstreamCircuitState, error) {
	var state AIGatewayUpstreamCircuitState
	err := s.db.Core.NewSelect().Model(&state).
		Where("upstream_id = ?", upstreamID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get aigateway upstream circuit state: %w", err)
	}
	return &state, nil
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) GetAllOpen(ctx context.Context) ([]AIGatewayUpstreamCircuitState, error) {
	var states []AIGatewayUpstreamCircuitState
	err := s.db.Core.NewSelect().Model(&states).
		Where("circuit_state = ?", "open").
		Scan(ctx)
	return states, err
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) GetAllClosed(ctx context.Context) ([]AIGatewayUpstreamCircuitState, error) {
	var states []AIGatewayUpstreamCircuitState
	err := s.db.Core.NewSelect().Model(&states).
		Where("circuit_state = ?", "closed").
		Scan(ctx)
	return states, err
}

func (s *aigatewayUpstreamCircuitStateStoreImpl) DeleteByUpstreamID(ctx context.Context, upstreamID int64) error {
	_, err := s.db.Core.NewDelete().Model((*AIGatewayUpstreamCircuitState)(nil)).
		Where("upstream_id = ?", upstreamID).
		Exec(ctx)
	return err
}
