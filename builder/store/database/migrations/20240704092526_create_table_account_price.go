package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AccountPrice struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	SkuType    int    `bun:",notnull" json:"sku_type"`
	SkuPrice   int64  `bun:",notnull" json:"sku_price"`
	SkuUnit    int64  `bun:",notnull" json:"sku_unit"`
	SkuDesc    string `bun:",notnull" json:"sku_desc"`
	ResourceID string `bun:",notnull" json:"resource_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountPrice{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountPrice{})
	})
}
