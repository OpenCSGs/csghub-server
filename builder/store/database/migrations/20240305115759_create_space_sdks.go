package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type SpaceSdk struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Name    string `bun:",notnull" json:"name"`
	Version string `bun:",notnull" json:"version"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, SpaceSdk{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SpaceSdk{})
	})
}
