package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type AccountBill struct {
	ID          int64     `bun:",pk,autoincrement" json:"id"`
	BillDate    time.Time `bun:"type:date" json:"bill_date"`
	UserID      string    `bun:",notnull" json:"user_id"`
	Scene       int       `bun:",notnull" json:"scene"`
	CustomerID  string    `bun:",notnull" json:"customer_id"`
	Value       float64   `bun:",notnull" json:"value"`
	Consumption float64   `bun:",notnull" json:"consumption"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountBill{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountBill{})
	})
}
