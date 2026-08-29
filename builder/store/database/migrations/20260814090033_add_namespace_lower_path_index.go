package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] ")
		_, err := db.ExecContext(ctx, `
			CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_namespaces_active_lower_path
			ON namespaces (LOWER(path))
			WHERE deleted_at IS NULL
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		_, err := db.ExecContext(ctx, `
			DROP INDEX CONCURRENTLY IF EXISTS idx_namespaces_active_lower_path
		`)
		return err
	})
}
