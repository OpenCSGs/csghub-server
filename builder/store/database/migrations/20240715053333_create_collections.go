package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

var collectionTables = []any{
	database.CollectionRepository{},
	database.Collection{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, collectionTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, collectionTables...)
	})
}
