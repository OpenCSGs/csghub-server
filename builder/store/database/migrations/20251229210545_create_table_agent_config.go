package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AgentConfig struct {
	ID     int64          `bun:",pk,autoincrement" json:"id"`
	Name   string         `bun:",notnull,unique" json:"name"`
	Config map[string]any `bun:",type:jsonb,notnull" json:"config"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AgentConfig{})
		if err != nil {
			return err
		}

		// Initialize with default values
		defaultConfig := AgentConfig{
			Name: "instance",
			Config: map[string]any{
				"code_instance_quota_per_user":     5,
				"langflow_instance_quota_per_user": 10,
			},
		}
		_, err = db.NewInsert().
			Model(&defaultConfig).
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AgentConfig{})
	})
}
