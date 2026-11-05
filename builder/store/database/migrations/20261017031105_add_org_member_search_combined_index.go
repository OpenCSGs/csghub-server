package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers the shared member search index for CE, EE, and SaaS.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Commit the extension separately: concurrent index DDL cannot run inside a transaction block.
		if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm"); err != nil {
			return fmt.Errorf("enable hierarchy member search trigrams: %w", err)
		}
		var exists, valid bool
		if err := db.QueryRowContext(ctx, `SELECT
			to_regclass('users_member_search_combined_idx') IS NOT NULL,
			COALESCE((SELECT indisvalid FROM pg_index
				WHERE indexrelid = to_regclass('users_member_search_combined_idx')
				AND indrelid = to_regclass('users')), false)`).Scan(&exists, &valid); err != nil {
			return fmt.Errorf("inspect hierarchy member search index: %w", err)
		}
		if valid {
			return nil
		}
		if exists {
			// Failed concurrent builds can leave invalid indexes that IF NOT EXISTS would silently retain.
			if _, err := db.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS users_member_search_combined_idx"); err != nil {
				return fmt.Errorf("drop invalid hierarchy member search index: %w", err)
			}
		}
		// Match the Store expression exactly; deleted users are excluded from both the query and index.
		_, err := db.ExecContext(ctx, `CREATE INDEX CONCURRENTLY users_member_search_combined_idx ON users USING gin
			((LOWER(COALESCE(username, '')) || E'\n' ||
			  LOWER(COALESCE(name, '')) || E'\n' ||
			  LOWER(COALESCE(email, '')) || E'\n' ||
			  LOWER(COALESCE(phone, ''))) gin_trgm_ops)
			WHERE deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("create concurrent hierarchy member search index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Retain pg_trgm because other indexes may also depend on this shared extension.
		if _, err := db.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS users_member_search_combined_idx"); err != nil {
			return fmt.Errorf("drop concurrent hierarchy member search index: %w", err)
		}
		return nil
	})
}
