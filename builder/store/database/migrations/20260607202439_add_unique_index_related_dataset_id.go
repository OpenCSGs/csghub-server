package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add_unique_index_related_dataset_id")
		_, err := db.ExecContext(ctx,
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_related_dataset_id ON datasets (related_dataset_id) WHERE related_dataset_id > 0")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] add_unique_index_related_dataset_id")
		_, err := db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_unique_related_dataset_id")
		return err
	})
}
