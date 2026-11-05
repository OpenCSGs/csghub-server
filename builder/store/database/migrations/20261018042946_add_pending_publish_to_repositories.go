package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Existing private repositories have no reliable publication intent and must stay private.
		_, err := db.ExecContext(ctx, "ALTER TABLE repositories ADD COLUMN pending_publish BOOLEAN NOT NULL DEFAULT FALSE")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		// Pause the publication schedule before rolling back to code using the legacy publication rule.
		_, err := db.ExecContext(ctx, "ALTER TABLE repositories DROP COLUMN pending_publish")
		return err
	})
}
