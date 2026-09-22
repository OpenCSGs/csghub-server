package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// init registers the active hierarchy root uniqueness constraint.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS organizations_single_hierarchical_root_idx
			ON organizations (is_root)
			WHERE is_root = TRUE
			  AND is_hierarchical = TRUE
			  AND deleted_at IS NULL
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			DROP INDEX IF EXISTS organizations_single_hierarchical_root_idx
		`)
		return err
	})
}
