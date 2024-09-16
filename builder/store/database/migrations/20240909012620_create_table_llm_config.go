package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type LLMConfig struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	ModelName   string `bun:",notnull" json:"model_name"`
	ApiEndpoint string `bun:",notnull" json:"api_endpoint"`
	AuthHeader  string `bun:",notnull" json:"auth_header"`
	Type        int    `bun:",notnull" json:"type"`
	Enabled     bool   `bun:",notnull" json:"enabled"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, LLMConfig{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, LLMConfig{})
	})
}
