package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// Create upstreams table and migrate data from llm_configs.jsonb upstreams
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Create upstreams table
			_, err := db.NewCreateTable().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstreams,alias:u"`

					ID                    int64     `bun:"id,pk,autoincrement"`
					LLMConfigID           int64     `bun:"llm_config_id,notnull"`
					URL                   string    `bun:"url,notnull"`
					Weight                int       `bun:"weight,notnull,default:1"`
					Enabled               bool      `bun:"enabled,notnull,default:true"`
					ModelName             string    `bun:"model_name,nullzero"`
					AuthHeader            string    `bun:"auth_header,nullzero"`
					Provider              string    `bun:"provider,nullzero"`
					HealthCheckEnabled    bool              `bun:"health_check_enabled,notnull,default:true"`
					CircuitBreakerEnabled bool              `bun:"circuit_breaker_enabled,notnull,default:true"`
					Tags                  map[string]string `bun:",type:jsonb,nullzero"`
					LimitPolicy           map[string]any    `bun:",type:jsonb,nullzero"`
					CreatedAt             time.Time         `bun:"created_at,notnull,default:current_timestamp"`
					UpdatedAt             time.Time         `bun:"updated_at,notnull,default:current_timestamp"`
				})(nil)).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return err
			}

			// Index on llm_config_id for relationship queries
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstreams"`
				})(nil)).
				Index("idx_ai_gateway_upstreams_llm_config_id").
				Column("llm_config_id").
				Exec(ctx)
			if err != nil {
				return err
			}

			// Unique constraint: same llm_config cannot have duplicate (model_name, provider)
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstreams"`
				})(nil)).
				Index("idx_ai_gateway_upstreams_llm_config_model_provider").
				Unique().
				Column("llm_config_id", "model_name", "provider").
				Exec(ctx)
			if err != nil {
				return err
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewDropTable().
				Model((*struct {
					bun.BaseModel `bun:"table:ai_gateway_upstreams"`
				})(nil)).
				IfExists().
				Exec(ctx)
			return err
		},
	)
}
