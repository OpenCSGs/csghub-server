package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init seeds the global administrator role and migrates legacy user roles for every edition.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Keep the role, its grant, and legacy assignments atomic without overwriting existing records.
			if _, err := tx.ExecContext(ctx, `INSERT INTO roles
				(role_code, display_name, description, is_admin, is_builtin, can_delete, created_at, user_id) VALUES
				('super_admin', '超级管理员', '总负责人、系统所有者', TRUE, TRUE, FALSE, CURRENT_TIMESTAMP, 0)
				ON CONFLICT (role_code) DO NOTHING`); err != nil {
				return fmt.Errorf("seed super administrator role: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO admin_role_permissions
				(role_code, permission_code, created_at) VALUES
				('super_admin', '*', CURRENT_TIMESTAMP)
				ON CONFLICT (role_code, permission_code) DO NOTHING`); err != nil {
				return fmt.Errorf("seed super administrator permission: %w", err)
			}
			// Match complete comma-separated roles and consolidate administrator aliases.
			// Use the user's update time and record the system operator as user ID 0.
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles
				(user_id, role_code, created_at, operator_user_id)
				SELECT DISTINCT u.id,
					CASE
						WHEN legacy.role_code IN ('super_user', 'admin', 'super_admin') THEN 'super_admin'
						ELSE legacy.role_code
					END,
					u.updated_at, 0
				FROM users AS u
				CROSS JOIN LATERAL UNNEST(STRING_TO_ARRAY(u.role_mask, ',')) AS legacy(role_code)
				WHERE legacy.role_code <> ''
				ON CONFLICT (user_id, role_code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
				return fmt.Errorf("migrate legacy user roles: %w", err)
			}
			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		// Preserve assignments and pre-existing records; the table migration removes them on schema rollback.
		return nil
	})
}
