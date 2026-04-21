package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] create_dataset_purchase_tasks")
		_, err := db.NewCreateTable().
			Model((*DatasetPurchaseTask)(nil)).
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] create_dataset_purchase_tasks")
		_, err := db.NewDropTable().
			Model((*DatasetPurchaseTask)(nil)).
			Exec(ctx)
		return err
	})
}

// DatasetPurchaseTask represents a dataset purchase task
type DatasetPurchaseTask struct {
	ID               int64  `bun:"id,pk,autoincrement"`
	DatasetPath      string `bun:"dataset_path,notnull"`
	DatasetID        int64  `bun:"dataset_id,notnull"`
	RelatedDatasetID int64  `bun:"related_dataset_id,notnull"`
	PurchaserID      int64  `bun:"purchaser_id,notnull"`
	TaskStatus       string `bun:"task_status,notnull"`
	times
}
