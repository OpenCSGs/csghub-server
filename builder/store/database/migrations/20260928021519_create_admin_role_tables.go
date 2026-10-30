package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers role definitions, user assignments, and administrator grants for every edition.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS roles (
				role_code VARCHAR(64) PRIMARY KEY CHECK (role_code <> ''),
				display_name VARCHAR(128) NOT NULL CHECK (display_name <> ''),
				description TEXT NOT NULL DEFAULT '',
				is_admin BOOLEAN NOT NULL DEFAULT FALSE,
				is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
				can_delete BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				user_id BIGINT NOT NULL
			)`); err != nil {
				return fmt.Errorf("create roles: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS admin_role_permissions (
				role_code VARCHAR(64) NOT NULL REFERENCES roles(role_code) ON DELETE CASCADE,
				permission_code VARCHAR(255) NOT NULL CHECK (permission_code <> ''),
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (role_code, permission_code)
			)`); err != nil {
				return fmt.Errorf("create admin_role_permissions: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS user_roles (
				id BIGSERIAL PRIMARY KEY,
				user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				role_code VARCHAR(64) NOT NULL CHECK (role_code <> ''),
				operator_user_id BIGINT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deleted_at TIMESTAMPTZ NULL
			)`); err != nil {
				return fmt.Errorf("create user_roles: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS user_roles_active_user_role_idx
				ON user_roles (user_id, role_code) WHERE deleted_at IS NULL`); err != nil {
				return fmt.Errorf("index active user roles: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS user_roles_active_role_user_idx
				ON user_roles (role_code, user_id) WHERE deleted_at IS NULL`); err != nil {
				return fmt.Errorf("index active role holders: %w", err)
			}
			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS user_roles"); err != nil {
				return fmt.Errorf("drop user_roles: %w", err)
			}
			if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS admin_role_permissions"); err != nil {
				return fmt.Errorf("drop admin_role_permissions: %w", err)
			}
			if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS roles"); err != nil {
				return fmt.Errorf("drop roles: %w", err)
			}
			return nil
		})
	})
}
