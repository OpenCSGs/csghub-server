package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// init allows new writers to omit real names while retaining legacy columns and values.
// TODO: Drop real_name and id_number_encrypted in the next release after old writers are retired.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "ALTER TABLE user_verifies ALTER COLUMN real_name SET DEFAULT ''")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		// Restore the original constraint behavior without changing any stored values.
		_, err := db.ExecContext(ctx, "ALTER TABLE user_verifies ALTER COLUMN real_name DROP DEFAULT")
		return err
	})
}
