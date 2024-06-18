package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, database.AccountEvent{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.AccountEvent{})
	})
}
