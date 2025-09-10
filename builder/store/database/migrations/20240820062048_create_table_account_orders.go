package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountOrder struct {
	OrderUUID   string            `bun:",notnull,pk" json:"order_uuid"`
	UserUUID    string            `bun:",notnull" json:"user_uuid"`
	OrderStatus types.OrderStatus `bun:",notnull" json:"order_status"`
	Amount      float64           `bun:",notnull" json:"amount"`
	CreatedAt   time.Time         `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	EventUUID   string            `json:"event_uuid"`
	RecordedAt  time.Time         `json:"recorded_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountOrder{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*AccountOrder)(nil)).
			Index("idx_account_order_createat_useruuid_status").
			Column("created_at", "user_uuid", "order_status").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountOrder{})
	})
}
