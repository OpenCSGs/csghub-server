package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Create composite index on user_uuid and created_at for AccountPresent table
		// This index will improve query performance for queries filtering by user_uuid and ordering by created_at
		// Using DESC order for created_at to optimize queries that fetch recent records
		_, err := db.NewCreateIndex().
			Model((*AccountPresent)(nil)).
			Index("idx_account_present_useruuid_createdat").
			Column("user_uuid", "created_at DESC").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Drop the index in the down migration
		_, err := db.NewDropIndex().
			Model((*AccountPresent)(nil)).
			Index("idx_account_present_useruuid_createdat").
			IfExists().
			Exec(ctx)
		return err
	})
}