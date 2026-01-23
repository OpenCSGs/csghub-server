package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type XnetMigrationTask struct {
	ID           int64  `bun:"id,pk,autoincrement"`
	RepositoryID int64  `bun:"repository_id,notnull"`
	LastMessage  string `bun:"last_message"`
	Status       string `bun:"status,notnull"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, XnetMigrationTask{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, XnetMigrationTask{})
	})
}
