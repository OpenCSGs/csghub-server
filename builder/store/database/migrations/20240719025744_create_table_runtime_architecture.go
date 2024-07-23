package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type RuntimeArchitecture struct {
	ID                 int64  `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64  `bun:",notnull" json:"runtime_framework_id"`
	ArchitectureName   string `bun:",notnull" json:"architecture_name"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, RuntimeArchitecture{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RuntimeArchitecture{})
	})
}
