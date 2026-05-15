package database

import (
	"context"
	"fmt"
	"time"
)

// AIGatewayUpstreamHealthState tracks health check state per upstream.
type AIGatewayUpstreamHealthState struct {
	ID                  int64          `bun:",pk,autoincrement" json:"id"`
	UpstreamID          int64          `bun:",unique,notnull" json:"upstream_id"`
	Upstream            *Upstream      `bun:"rel:belongs-to,join:upstream_id=id" json:"-"`
	HealthState         string         `bun:",notnull,default:healthy" json:"health_state"` // healthy, degraded, unhealthy
	LastCheckAt         time.Time      `bun:",notnull" json:"last_check_at"`
	LastError           string         `bun:",nullzero" json:"last_error"`
	ConsecutiveFailures int            `bun:",notnull,default:0" json:"consecutive_failures"`
	LatencyMs           int64          `bun:",notnull,default:0" json:"latency_ms"`
	Metadata            map[string]any `bun:",type:jsonb,nullzero" json:"metadata"`
	times
}

type AIGatewayUpstreamHealthStateStore interface {
	Create(ctx context.Context, state *AIGatewayUpstreamHealthState) error
	Update(ctx context.Context, state *AIGatewayUpstreamHealthState) error
	Upsert(ctx context.Context, state *AIGatewayUpstreamHealthState) error
	GetByUpstreamID(ctx context.Context, upstreamID int64) (*AIGatewayUpstreamHealthState, error)
	GetAllHealthy(ctx context.Context) ([]AIGatewayUpstreamHealthState, error)
	GetAllUnhealthy(ctx context.Context) ([]AIGatewayUpstreamHealthState, error)
	DeleteByUpstreamID(ctx context.Context, upstreamID int64) error
}

type aigatewayUpstreamHealthStateStoreImpl struct {
	db *DB
}

func NewAIGatewayUpstreamHealthStateStore() AIGatewayUpstreamHealthStateStore {
	return &aigatewayUpstreamHealthStateStoreImpl{db: defaultDB}
}

func NewAIGatewayUpstreamHealthStateStoreWithDB(db *DB) AIGatewayUpstreamHealthStateStore {
	return &aigatewayUpstreamHealthStateStoreImpl{db: db}
}

func (s *aigatewayUpstreamHealthStateStoreImpl) Create(ctx context.Context, state *AIGatewayUpstreamHealthState) error {
	_, err := s.db.Core.NewInsert().Model(state).Exec(ctx)
	return err
}

func (s *aigatewayUpstreamHealthStateStoreImpl) Update(ctx context.Context, state *AIGatewayUpstreamHealthState) error {
	_, err := s.db.Core.NewUpdate().Model(state).Where("id = ?", state.ID).Exec(ctx)
	return err
}

func (s *aigatewayUpstreamHealthStateStoreImpl) Upsert(ctx context.Context, state *AIGatewayUpstreamHealthState) error {
	_, err := s.db.Core.NewInsert().
		Model(state).
		On("CONFLICT (upstream_id) DO UPDATE").
		Set("health_state = EXCLUDED.health_state").
		Set("last_check_at = EXCLUDED.last_check_at").
		Set("last_error = EXCLUDED.last_error").
		Set("consecutive_failures = EXCLUDED.consecutive_failures").
		Set("latency_ms = EXCLUDED.latency_ms").
		Set("metadata = EXCLUDED.metadata").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}

func (s *aigatewayUpstreamHealthStateStoreImpl) GetByUpstreamID(ctx context.Context, upstreamID int64) (*AIGatewayUpstreamHealthState, error) {
	var state AIGatewayUpstreamHealthState
	err := s.db.Core.NewSelect().Model(&state).
		Where("upstream_id = ?", upstreamID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get aigateway upstream health state: %w", err)
	}
	return &state, nil
}

func (s *aigatewayUpstreamHealthStateStoreImpl) GetAllHealthy(ctx context.Context) ([]AIGatewayUpstreamHealthState, error) {
	var states []AIGatewayUpstreamHealthState
	err := s.db.Core.NewSelect().Model(&states).
		Where("health_state = ?", "healthy").
		Scan(ctx)
	return states, err
}

func (s *aigatewayUpstreamHealthStateStoreImpl) GetAllUnhealthy(ctx context.Context) ([]AIGatewayUpstreamHealthState, error) {
	var states []AIGatewayUpstreamHealthState
	err := s.db.Core.NewSelect().Model(&states).
		Where("health_state = ?", "unhealthy").
		Scan(ctx)
	return states, err
}

func (s *aigatewayUpstreamHealthStateStoreImpl) DeleteByUpstreamID(ctx context.Context, upstreamID int64) error {
	_, err := s.db.Core.NewDelete().Model((*AIGatewayUpstreamHealthState)(nil)).
		Where("upstream_id = ?", upstreamID).
		Exec(ctx)
	return err
}
