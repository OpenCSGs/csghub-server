package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers the reversible mirror task urgency column migration.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE mirror_tasks
			ADD COLUMN IF NOT EXISTS is_urgent BOOLEAN NOT NULL DEFAULT FALSE
		`); err != nil {
			return fmt.Errorf("failed to add mirror task is_urgent column: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE mirror_tasks
			DROP COLUMN IF EXISTS is_urgent
		`); err != nil {
			return fmt.Errorf("failed to drop mirror task is_urgent column: %w", err)
		}
		return nil
	})
}
