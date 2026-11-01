package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE repositories
				ADD COLUMN IF NOT EXISTS xnet_migration_intent VARCHAR NOT NULL DEFAULT 'inactive';
			ALTER TABLE lfs_meta_objects
				ADD COLUMN IF NOT EXISTS xnet_dispatch_status VARCHAR NOT NULL DEFAULT 'pending',
				ADD COLUMN IF NOT EXISTS xnet_dispatched_at TIMESTAMPTZ NULL,
				ADD COLUMN IF NOT EXISTS xnet_next_attempt_at TIMESTAMPTZ NULL,
				ADD COLUMN IF NOT EXISTS xnet_dispatch_attempts INTEGER NOT NULL DEFAULT 0
		`); err != nil {
			return err
		}
		// Already migrated rows are excluded by xnet_used, so no table-wide status
		// backfill is needed. Keep the index build outside a multi-statement query:
		// PostgreSQL otherwise creates an implicit transaction and rejects CONCURRENTLY.
		// Preserve completed index builds when startup times out midway through this
		// migration; only remove INVALID artifacts left by failed concurrent builds.
		if err := dropInvalidXnetDispatchIndex(ctx, db, "idx_lfs_meta_objects_xnet_dispatch"); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lfs_meta_objects_xnet_dispatch
				ON lfs_meta_objects (xnet_dispatch_status, xnet_next_attempt_at, xnet_dispatched_at, id)
				WHERE xnet_used = FALSE
		`); err != nil {
			return err
		}
		if err := dropInvalidXnetDispatchIndex(ctx, db, "idx_repositories_xnet_migration_intent"); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_xnet_migration_intent
				ON repositories (xnet_migration_intent, id)
		`); err != nil {
			return err
		}
		if err := dropInvalidXnetDispatchIndex(ctx, db, "idx_lfs_meta_objects_repository_xnet_dispatch"); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lfs_meta_objects_repository_xnet_dispatch
				ON lfs_meta_objects (repository_id, xnet_dispatch_status, xnet_next_attempt_at, xnet_dispatched_at, id)
				WHERE xnet_used = FALSE
		`); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS xnet_dispatch_rate_limits (
				name VARCHAR PRIMARY KEY,
				next_allowed_at TIMESTAMPTZ NOT NULL
			)
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			DROP INDEX CONCURRENTLY IF EXISTS idx_lfs_meta_objects_repository_xnet_dispatch
		`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			DROP INDEX CONCURRENTLY IF EXISTS idx_repositories_xnet_migration_intent
		`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			DROP INDEX CONCURRENTLY IF EXISTS idx_lfs_meta_objects_xnet_dispatch
		`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			DROP TABLE IF EXISTS xnet_dispatch_rate_limits
		`); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `
			ALTER TABLE repositories
				DROP COLUMN IF EXISTS xnet_migration_intent;
			ALTER TABLE lfs_meta_objects
				DROP COLUMN IF EXISTS xnet_dispatch_attempts,
				DROP COLUMN IF EXISTS xnet_next_attempt_at,
				DROP COLUMN IF EXISTS xnet_dispatched_at,
				DROP COLUMN IF EXISTS xnet_dispatch_status
		`)
		return err
	})
}

// dropInvalidXnetDispatchIndex preserves valid builds across migration retries.
// Each concurrent DDL statement must execute outside a transaction.
func dropInvalidXnetDispatchIndex(ctx context.Context, db *bun.DB, name string) error {
	var invalid bool
	if err := db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_index WHERE indexrelid = to_regclass(?) AND NOT indisvalid
	)`, name).Scan(ctx, &invalid); err != nil {
		return fmt.Errorf("inspect Xnet dispatch index %s: %w", name, err)
	}
	if !invalid {
		return nil
	}
	if _, err := db.NewRaw("DROP INDEX CONCURRENTLY IF EXISTS ?", bun.Ident(name)).Exec(ctx); err != nil {
		return fmt.Errorf("drop invalid Xnet dispatch index %s: %w", name, err)
	}
	return nil
}
