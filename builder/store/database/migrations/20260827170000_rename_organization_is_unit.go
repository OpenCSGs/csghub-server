package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers the migration that renames the organization hierarchy flag.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE organizations
			RENAME COLUMN is_unit TO is_hierarchical
		`); err != nil {
			return fmt.Errorf("rename organizations.is_unit to is_hierarchical: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE organizations
			RENAME COLUMN is_hierarchical TO is_unit
		`); err != nil {
			return fmt.Errorf("rename organizations.is_hierarchical to is_unit: %w", err)
		}
		return nil
	})
}
