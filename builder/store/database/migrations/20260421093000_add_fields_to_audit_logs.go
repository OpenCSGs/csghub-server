package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add_fields_to_audit_logs")
		queries := []string{
			"ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_name VARCHAR",
			"ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS bearer_token VARCHAR",
			"ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS ip_address VARCHAR",
			"ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS auth_type VARCHAR",
			"ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS resource_id VARCHAR",
		}
		for _, query := range queries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] add_fields_to_audit_logs")
		queries := []string{
			"ALTER TABLE audit_logs DROP COLUMN IF EXISTS resource_id",
			"ALTER TABLE audit_logs DROP COLUMN IF EXISTS auth_type",
			"ALTER TABLE audit_logs DROP COLUMN IF EXISTS ip_address",
			"ALTER TABLE audit_logs DROP COLUMN IF EXISTS bearer_token",
			"ALTER TABLE audit_logs DROP COLUMN IF EXISTS user_name",
		}
		for _, query := range queries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	})
}
