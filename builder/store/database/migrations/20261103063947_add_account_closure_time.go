package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN closure_requested_at TIMESTAMPTZ;
			UPDATE users SET closure_requested_at = CURRENT_TIMESTAMP WHERE login_frozen`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "ALTER TABLE users DROP COLUMN closure_requested_at")
		return err
	})
}
