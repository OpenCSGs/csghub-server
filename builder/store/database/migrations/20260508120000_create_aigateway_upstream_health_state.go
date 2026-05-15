package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// Create aigateway upstream health state and circuit breaker state tables
// with upstream_id as the unique key instead of (provider, model_name, endpoint).
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Create ai_gateway_upstream_health_states table
			_, err := db.NewCreateTable().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_health_states,alias:phs"`

					ID                  int64          `bun:"id,pk,autoincrement"`
					UpstreamID          int64          `bun:"upstream_id,unique,notnull"`
					HealthState         string         `bun:"health_state,notnull,default:'healthy'"` // healthy, degraded, unhealthy
					LastCheckAt         time.Time      `bun:"last_check_at,notnull"`
					LastError           string         `bun:"last_error,nullzero"`
					ConsecutiveFailures int            `bun:"consecutive_failures,notnull,default:0"`
					LatencyMs           int64          `bun:"latency_ms,notnull,default:0"`
					Metadata            map[string]any `bun:"metadata,type:jsonb,nullzero"`
					times
				})(nil)).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return err
			}

			// Create index on health_state for quick filtering
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_health_states"`
				})(nil)).
				Index("idx_ai_gateway_upstream_health_states_health_state").
				Column("health_state").
				Exec(ctx)
			if err != nil {
				return err
			}

			// Create index on upstream_id for joins
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_health_states"`
				})(nil)).
				Index("idx_ai_gateway_upstream_health_states_upstream_id").
				Column("upstream_id").
				Exec(ctx)
			if err != nil {
				return err
			}

			// Create ai_gateway_upstream_circuit_states table
			_, err = db.NewCreateTable().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_circuit_states,alias:pcs"`

					ID              int64          `bun:"id,pk,autoincrement"`
					UpstreamID      int64          `bun:"upstream_id,unique,notnull"`
					CircuitState    string         `bun:"circuit_state,notnull,default:'closed'"` // closed, open, half_open
					FailureCount    int            `bun:"failure_count,notnull,default:0"`
					SuccessCount    int            `bun:"success_count,notnull,default:0"`
					LastStateChange time.Time      `bun:"last_state_change,notnull"`
					NextRetryAt     *time.Time     `bun:"next_retry_at,nullzero"`
					Metadata        map[string]any `bun:"metadata,type:jsonb,nullzero"`
					times
				})(nil)).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return err
			}

			// Create index on circuit_state for quick filtering
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_circuit_states"`
				})(nil)).
				Index("idx_ai_gateway_upstream_circuit_states_circuit_state").
				Column("circuit_state").
				Exec(ctx)
			if err != nil {
				return err
			}

			// Create index on upstream_id for joins
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstream_circuit_states"`
				})(nil)).
				Index("idx_ai_gateway_upstream_circuit_states_upstream_id").
				Column("upstream_id").
				Exec(ctx)
			if err != nil {
				return err
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, _ = db.NewDropTable().
				Model((*struct{ bun.BaseModel `bun:"table:ai_gateway_upstream_circuit_states"` })(nil)).
				IfExists().
				Exec(ctx)
			_, _ = db.NewDropTable().
				Model((*struct{ bun.BaseModel `bun:"table:ai_gateway_upstream_health_states"` })(nil)).
				IfExists().
				Exec(ctx)
			return nil
		},
	)
}
