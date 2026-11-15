package migrations

import (
	"context"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "ALTER TABLE lfs_meta_objects ADD COLUMN latest_commit_id TEXT NOT NULL DEFAULT ''")
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE mirror_tasks ADD COLUMN latest_lfs_scan_commit_id TEXT NOT NULL DEFAULT ''")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "ALTER TABLE mirror_tasks DROP COLUMN latest_lfs_scan_commit_id")
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE lfs_meta_objects DROP COLUMN latest_commit_id")
		return err
	})
}
