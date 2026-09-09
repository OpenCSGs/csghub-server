package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers hierarchy metadata, active-path uniqueness, and membership constraints.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		statements := []string{
			`ALTER TABLE organizations
				ADD COLUMN IF NOT EXISTS is_root BOOLEAN NOT NULL DEFAULT TRUE,
				ADD COLUMN IF NOT EXISTS is_unit BOOLEAN NOT NULL DEFAULT FALSE,
				ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL`,
			`DROP INDEX IF EXISTS idx_namespaces_path`,
			`CREATE UNIQUE INDEX idx_namespaces_path
				ON namespaces (path)
				WHERE deleted_at IS NULL`,
			`DROP INDEX IF EXISTS idx_organizations_path`,
			`CREATE UNIQUE INDEX idx_organizations_path
				ON organizations (path)
				WHERE deleted_at IS NULL`,
			`DO $$
			BEGIN
				IF to_regclass('organization_units') IS NOT NULL THEN
					UPDATE organizations
					SET is_unit = TRUE
					WHERE id IN (SELECT organization_id FROM organization_units);
				END IF;
			END $$`,
			`ALTER TABLE members DROP CONSTRAINT members_pkey`,
			`ALTER TABLE members ADD CONSTRAINT members_pkey PRIMARY KEY (id)`,
			`ALTER TABLE members ADD CONSTRAINT members_role_check
				CHECK (role IN ('admin', 'write', 'read'))`,
			`CREATE UNIQUE INDEX members_organization_user_active_unique_idx
				ON members (organization_id, user_id)
				WHERE deleted_at IS NULL`,
			`CREATE INDEX members_user_organization_active_idx
				ON members (user_id, organization_id)
				WHERE deleted_at IS NULL`,
		}
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, statement := range statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("adjust organization hierarchy schema: %w", err)
				}
			}
			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		statements := []string{
			`DROP INDEX IF EXISTS idx_organizations_path`,
			`CREATE UNIQUE INDEX idx_organizations_path ON organizations (path)`,
			`DROP INDEX IF EXISTS idx_namespaces_path`,
			`CREATE UNIQUE INDEX idx_namespaces_path ON namespaces (path)`,
			`DO $$
			BEGIN
				IF to_regclass('organization_units') IS NOT NULL THEN
					UPDATE organizations
					SET is_unit = FALSE
					WHERE id IN (SELECT organization_id FROM organization_units);
				END IF;
			END $$`,
			`DROP INDEX members_user_organization_active_idx`,
			`DROP INDEX members_organization_user_active_unique_idx`,
			`ALTER TABLE members DROP CONSTRAINT members_role_check`,
			`ALTER TABLE members DROP CONSTRAINT members_pkey`,
			`ALTER TABLE members ADD CONSTRAINT members_pkey PRIMARY KEY (id, organization_id, user_id)`,
			`ALTER TABLE organizations
				DROP COLUMN IF EXISTS deleted_at,
				DROP COLUMN IF EXISTS is_root,
				DROP COLUMN IF EXISTS is_unit`,
		}
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, statement := range statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("restore organization hierarchy schema: %w", err)
				}
			}
			return nil
		})
	})
}
