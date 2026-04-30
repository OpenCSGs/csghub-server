package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] alter_audit_logs_before_after_to_jsonb")
		queries := []string{
			`ALTER TABLE audit_logs
				ALTER COLUMN before TYPE jsonb
				USING CASE WHEN before IS NULL OR btrim(before) = '' THEN 'null'::jsonb ELSE before::jsonb END`,
			`ALTER TABLE audit_logs
				ALTER COLUMN after TYPE jsonb
				USING CASE WHEN after IS NULL OR btrim(after) = '' THEN 'null'::jsonb ELSE after::jsonb END`,
			`ALTER TABLE audit_logs
				ALTER COLUMN operator_id TYPE TEXT
				USING operator_id::TEXT;`,
		}
		for _, query := range queries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] alter_audit_logs_before_after_to_jsonb")
		queries := []string{
			"ALTER TABLE audit_logs ALTER COLUMN after TYPE text USING after::text",
			"ALTER TABLE audit_logs ALTER COLUMN before TYPE text USING before::text",
			"ALTER TABLE audit_logs ALTER COLUMN operator_id TYPE INTEGER USING operator_id::INTEGER",
		}
		for _, query := range queries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	})
}
