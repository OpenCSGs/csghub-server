package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type PendingDeletion struct {
	ID        int64  `bun:",pk,autoincrement"`
	TableName string `bun:",notnull"`
	Value     string `bun:",notnull"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, &PendingDeletion{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &PendingDeletion{})
	})
}
