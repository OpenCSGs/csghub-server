package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// Migrate LLMConfig upstreams from JSONB into upstreams table and make ModelName unique.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// 1. Handle duplicate ModelName: append -{id} suffix
			type dupRow struct {
				ID        int64
				ModelName string
			}
			var dups []dupRow
			err := db.NewRaw(`
				SELECT id, model_name FROM (
					SELECT id, model_name,
						ROW_NUMBER() OVER (PARTITION BY model_name ORDER BY id) AS rn
					FROM llm_configs
				) sub WHERE rn > 1
			`).Scan(ctx, &dups)
			if err != nil {
				return fmt.Errorf("failed to find duplicate model names: %w", err)
			}
			for _, d := range dups {
				_, err := db.NewRaw(
					`UPDATE llm_configs SET model_name = ? WHERE id = ?`,
					fmt.Sprintf("%s-%d", d.ModelName, d.ID),
					d.ID,
				).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to rename duplicate model_name: %w", err)
				}
			}

			// 2. Migrate upstreams from JSONB to upstreams table
			type llmRow struct {
				ID        int64
				Upstreams sql.NullString
			}
			var rows []llmRow
			err = db.NewRaw(`SELECT id, upstreams FROM llm_configs WHERE upstreams IS NOT NULL`).Scan(ctx, &rows)
			if err != nil {
				return fmt.Errorf("failed to load llm_config upstreams: %w", err)
			}

			type upstreamConfig struct {
				URL                   string            `json:"url"`
				Weight                int               `json:"weight,omitempty"`
				Enabled               bool              `json:"enabled"`
				ModelName             string            `json:"model_name,omitempty"`
				AuthHeader            string            `json:"auth_header,omitempty"`
				Provider              string            `json:"provider,omitempty"`
				HealthCheckEnabled    *bool             `json:"health_check_enabled,omitempty"`
				CircuitBreakerEnabled *bool             `json:"circuit_breaker_enabled,omitempty"`
				Tags                  map[string]string `json:"tags,omitempty"`
				LimitPolicy           json.RawMessage   `json:"limit_policy,omitempty"`
			}

			for _, row := range rows {
				if !row.Upstreams.Valid || strings.TrimSpace(row.Upstreams.String) == "" || row.Upstreams.String == "null" {
					continue
				}
				var upstreams []upstreamConfig
				if err := json.Unmarshal([]byte(row.Upstreams.String), &upstreams); err != nil {
					return fmt.Errorf("failed to unmarshal upstreams for llm_config id %d: %w", row.ID, err)
				}
				for _, u := range upstreams {
					u.URL = strings.TrimSpace(u.URL)
					if u.URL == "" {
						continue
					}
					if u.Weight <= 0 {
						u.Weight = 1
					}

					healthEnabled := true
					if u.HealthCheckEnabled != nil {
						healthEnabled = *u.HealthCheckEnabled
					}
					circuitEnabled := true
					if u.CircuitBreakerEnabled != nil {
						circuitEnabled = *u.CircuitBreakerEnabled
					}

					// Marshal tags and limit_policy as JSON for the JSONB columns
					var tagsJSON, limitJSON *string
					if len(u.Tags) > 0 {
						b, _ := json.Marshal(u.Tags)
						s := string(b)
						tagsJSON = &s
					}
					if len(u.LimitPolicy) > 0 && string(u.LimitPolicy) != "null" {
						s := string(u.LimitPolicy)
						limitJSON = &s
					}

					_, err := db.NewRaw(
						`INSERT INTO ai_gateway_upstreams (llm_config_id, url, weight, enabled, model_name, auth_header, provider, health_check_enabled, circuit_breaker_enabled, tags, limit_policy, created_at, updated_at)
						 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, NOW(), NOW())`,
						row.ID, u.URL, u.Weight, u.Enabled, nullStr(u.ModelName), nullStr(u.AuthHeader), nullStr(u.Provider),
						healthEnabled, circuitEnabled, tagsJSON, limitJSON,
					).Exec(ctx)
					if err != nil {
						return fmt.Errorf("failed to insert upstream for llm_config %d url %s: %w", row.ID, u.URL, err)
					}
				}
			}

			// 3. Add unique constraint on model_name
			_, err = db.NewCreateIndex().
				Model((*struct {
					bun.BaseModel `bun:"table:llm_configs"`
				})(nil)).
				Index("idx_llm_configs_model_name_unique").
				Unique().
				Column("model_name").
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return err
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, _ = db.NewDropIndex().
				Model((*struct{ bun.BaseModel `bun:"table:llm_configs"` })(nil)).
				Index("idx_llm_configs_model_name_unique").
				IfExists().
				Exec(ctx)
			return nil
		},
	)
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
