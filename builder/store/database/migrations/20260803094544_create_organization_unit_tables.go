package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// init registers the organization unit schema migration.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		statements := []string{
			`CREATE TABLE organization_units (
				id BIGSERIAL PRIMARY KEY,
				root_organization_id BIGINT NOT NULL,
				organization_id BIGINT NOT NULL,
				parent_unit_id BIGINT NULL,
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deleted_at TIMESTAMPTZ NULL,
				CONSTRAINT organization_units_not_self_parent
					CHECK (parent_unit_id IS NULL OR parent_unit_id <> id)
			)`,
			`CREATE UNIQUE INDEX organization_units_organization_active_unique_idx
				ON organization_units (organization_id)
				WHERE deleted_at IS NULL`,
			`CREATE INDEX organization_units_roots_idx
				ON organization_units (root_organization_id, sort_order, id)
				WHERE parent_unit_id IS NULL AND deleted_at IS NULL`,
			`CREATE INDEX organization_units_children_idx
				ON organization_units (root_organization_id, parent_unit_id, sort_order, id)
				WHERE deleted_at IS NULL`,
			`CREATE TABLE organization_unit_closure (
				root_organization_id BIGINT NOT NULL,
				ancestor_unit_id BIGINT NOT NULL,
				descendant_unit_id BIGINT NOT NULL,
				depth INTEGER NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (root_organization_id, ancestor_unit_id, descendant_unit_id),
				CONSTRAINT organization_unit_closure_nonnegative_depth CHECK (depth >= 0),
				CONSTRAINT organization_unit_closure_self_depth CHECK (
					(depth = 0 AND ancestor_unit_id = descendant_unit_id) OR
					(depth > 0 AND ancestor_unit_id <> descendant_unit_id)
				)
			)`,
			`CREATE INDEX organization_unit_closure_ancestors_idx
				ON organization_unit_closure (root_organization_id, descendant_unit_id, depth, ancestor_unit_id)`,
			`CREATE INDEX organization_unit_closure_subtree_idx
				ON organization_unit_closure (root_organization_id, ancestor_unit_id, depth, descendant_unit_id)`,
		}
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, statement := range statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("create organization unit schema: %w", err)
				}
			}
			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		statements := []string{
			"DROP TABLE IF EXISTS organization_unit_closure",
			"DROP TABLE IF EXISTS organization_units",
		}
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, statement := range statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("drop organization unit schema: %w", err)
				}
			}
			return nil
		})
	})
}
