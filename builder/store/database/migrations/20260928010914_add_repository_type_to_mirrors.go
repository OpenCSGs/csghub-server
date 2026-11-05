package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] ")
		_, err := db.ExecContext(ctx, `ALTER TABLE mirrors ADD COLUMN repository_type VARCHAR(32)`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		_, err := db.ExecContext(ctx, `ALTER TABLE mirrors DROP COLUMN repository_type`)
		return err
	})
}
