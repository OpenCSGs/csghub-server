package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountStatementDailySummary struct {
	ID               int64           `bun:",pk,autoincrement" json:"id"`
	BillDate         time.Time       `bun:"type:date,notnull" json:"bill_date"`
	UserUUID         string          `bun:",notnull" json:"user_uuid"`
	SkuID            int64           `bun:",notnull,default:0" json:"sku_id"`
	Scene            types.SceneType `bun:",notnull" json:"scene"`
	CustomerID       string          `bun:",notnull,default:''" json:"customer_id"`
	TotalValue       float64         `bun:",nullzero" json:"total_value"`
	TotalConsumption float64         `bun:",nullzero" json:"total_consumption"`
	TotalCount       int64           `bun:",nullzero" json:"total_count"`
	MinID            int64           `bun:",nullzero" json:"min_id"`
	MinCreatedAt     time.Time       `bun:"type:timestamp,nullzero" json:"min_created_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountStatementDailySummary{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountStatementDailySummary{})
	})
}
