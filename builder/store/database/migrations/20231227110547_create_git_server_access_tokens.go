package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, GitServerAccessToken{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, GitServerAccessToken{})
	})
}

type GitServerAccessToken struct {
	ID    int64  `bun:",pk,autoincrement" json:"id"`
	Token string `bun:",notnull" json:"token"`
	times
}
