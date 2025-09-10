package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountOrderDetail struct {
	ID          int64         `bun:",pk,autoincrement" json:"id"`
	OrderUUID   string        `bun:",notnull" json:"order_uuid"`
	ResourceID  string        `bun:",notnull" json:"resource_id"`
	SkuType     types.SKUType `bun:",notnull" json:"sku_type"`
	SkuKind     types.SKUKind `bun:",notnull" json:"sku_kind"`
	SkuUnitType string        `bun:",notnull" json:"sku_unit_type"`
	OrderCount  int           `bun:",notnull" json:"order_count"`
	SkuPriceID  int64         `bun:",notnull" json:"sku_price_id"`
	Amount      float64       `bun:",notnull" json:"amount"`
	BeginTime   time.Time     `bun:",notnull" json:"begin_time"`
	EndTime     time.Time     `bun:",notnull" json:"end_time"`
	CreatedAt   time.Time     `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	PresentUUID string        `json:"present_uuid"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountOrderDetail{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*AccountOrderDetail)(nil)).
			Index("idx_account_order_detail_orderuuid").
			Column("order_uuid").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountOrderDetail{})
	})
}
