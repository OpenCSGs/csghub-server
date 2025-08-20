package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type BillSummaryDB struct {
	bun.BaseModel `bun:"table:payment_bill_summary,alias:pbs"`

	ID          int64     `bun:",pk,autoincrement" json:"id"`
	GatewayType string    `bun:",notnull" json:"gateway_type"`
	Account     string    `bun:",notnull" json:"account"`
	BillDate    time.Time `bun:",notnull,type:timestamp" json:"bill_date"`
	S3Bucket    string    `bun:",notnull" json:"s3_bucket"`
	S3Key       string    `bun:",notnull" json:"s3_key"`

	CreatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"updated_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, BillSummaryDB{}); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model(&BillSummaryDB{}).
			Index("idx_payment_bill_summary_unique").
			Unique().
			Column("gateway_type", "account", "bill_date").
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, BillSummaryDB{})
	})
}
