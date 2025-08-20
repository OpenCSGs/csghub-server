package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type BillDetailDB struct {
	bun.BaseModel `bun:"table:payment_bill_detail,alias:pbd"`

	ID              int64     `bun:",pk,autoincrement" json:"id"`
	BillSummaryID   int64     `bun:",notnull" json:"bill_summary_id"`
	PayOrderID      string    `bun:",notnull" json:"pay_order_id"`
	MerchantOrderID string    `bun:",notnull" json:"merchant_order_id"`
	BusinessType    string    `bun:",notnull" json:"business_type"`
	ProductName     string    `bun:",notnull" json:"product_name"`
	CreateTime      time.Time `bun:",notnull" json:"create_time"`
	CompleteTime    time.Time `bun:",notnull" json:"complete_time"`
	PayUser         string    `bun:",notnull" json:"pay_user"`
	OrderAmount     float64   `bun:",notnull" json:"order_amount"`
	MerchantReceive float64   `bun:",notnull" json:"merchant_receive"`
	ServiceFee      float64   `bun:",notnull" json:"service_fee"`
	Currency        string    `bun:",nullzero,default:'CNY'" json:"currency"`
	Remark          string    `json:"remark"`
	CreatedAt       time.Time `bun:",nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt       time.Time `bun:",nullzero,default:current_timestamp" json:"updated_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, BillDetailDB{}); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model(&BillDetailDB{}).
			Index("idx_payment_bill_detail_unique").
			Unique().
			Column("bill_summary_id", "pay_order_id").
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, BillDetailDB{})
	})
}
