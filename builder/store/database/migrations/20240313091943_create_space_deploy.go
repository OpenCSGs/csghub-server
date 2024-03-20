package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		dropTables(ctx, db, database.Space{})
		return createTables(ctx, db, database.Space{}, database.Deploy{}, database.DeployTask{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.Deploy{}, database.DeployTask{})
	})
}
