package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

var orgTables = []any{
	database.Organization{},
	database.Member{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, orgTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, orgTables...)
	})
}
