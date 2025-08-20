package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, AccessToken{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccessToken{})
	})
}
