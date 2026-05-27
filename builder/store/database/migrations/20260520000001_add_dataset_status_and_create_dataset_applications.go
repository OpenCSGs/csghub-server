package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type DatasetApplication struct {
	ID               int64   `bun:"id,pk,autoincrement"`
	DatasetID        int64   `bun:"dataset_id,notnull"`
	ApplicantID      int64   `bun:"applicant_id,notnull"`
	Action           string  `bun:"action,notnull"`
	Price            float64 `bun:"price"`
	RelatedDatasetID int64   `bun:"related_dataset_id"`
	Status           string  `bun:"status,notnull,default:'pending'"`
	ReviewerID       int64   `bun:"reviewer_id"`
	ReviewMsg        string  `bun:"review_msg"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add_dataset_status_and_create_dataset_applications")
		// Add status column to datasets table
		_, err := db.ExecContext(ctx, "ALTER TABLE datasets ADD COLUMN IF NOT EXISTS status VARCHAR(255) NOT NULL DEFAULT 'normal'")
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE datasets ADD COLUMN IF NOT EXISTS current_application_id BIGINT")
		if err != nil {
			return err
		}
		// Create dataset_applications table
		if err := createTables(ctx, db, DatasetApplication{}); err != nil {
			return err
		}
		// Add partial unique index to prevent duplicate pending applications
		_, err = db.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_one_pending_per_dataset ON dataset_applications (dataset_id) WHERE status = 'pending'")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] add_dataset_status_and_create_dataset_applications")
		// Drop dataset_applications table
		if _, err := db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_one_pending_per_dataset"); err != nil {
			return err
		}
		if err := dropTables(ctx, db, DatasetApplication{}); err != nil {
			return err
		}
		// Drop columns from datasets table
		if _, err := db.ExecContext(ctx, "ALTER TABLE datasets DROP COLUMN IF EXISTS current_application_id"); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, "ALTER TABLE datasets DROP COLUMN IF EXISTS status")
		return err
	})
}
