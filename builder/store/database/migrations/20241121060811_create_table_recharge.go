package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type AccountRecharge struct {
	RechargeUUID  string    `bun:",notnull,pk,skipupdate" json:"uuid"`                // Recharge object ID
	OrderNo       string    `bun:",notnull,unique" json:"order_no"`                   // Order ID allowed by the payment system
	UserUUID      string    `bun:",notnull,skipupdate" json:"user_uuid"`              // Target UserUUID for the recharge
	FromUserUUID  string    `bun:",notnull,skipupdate" json:"from_user_uuid"`         // Source UserUUID for the recharge
	Amount        float64   `bun:",notnull,skipupdate" json:"amount"`                 // Actual balance received by the user, in cents
	Currency      string    `bun:",notnull,skipupdate,default:'CNY'" json:"currency"` // 3-letter ISO currency code in uppercase letters
	Payment       *Payment  `bun:"rel:belongs-to,join:payment_uuid=payment_uuid" json:"payment"`
	PaymentUUID   string    `bun:",notnull,skipupdate,unique" json:"payment_uuid"`
	Succeeded     bool      `json:"succeeded"`
	Closed        bool      `json:"closed"`
	TimeSucceeded time.Time `bun:",nullzero" json:"time_succeeded"`
	CreatedAt     time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time `bun:",notnull,default:current_timestamp" json:"updated_at"`
	Description   string    `json:"description"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountRecharge{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*AccountRecharge)(nil)).
			Index("idx_payment_uuid").
			Column("payment_uuid").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_payment_uuid fail: %w", err)
		}
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountRecharge{})
	})
}
