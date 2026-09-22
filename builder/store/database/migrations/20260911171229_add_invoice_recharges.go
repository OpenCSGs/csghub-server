package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// AccountInvoiceRecharge links an invoice to the recharge orders invoiced together.
type AccountInvoiceRecharge struct {
	ID        int64 `bun:",pk,autoincrement"`
	InvoiceID int64 `bun:"invoice_id,notnull"`
	// A recharge order may appear in multiple invoices over its lifetime (a
	// failed invoice releases the order for a new application), so this is a
	// plain non-unique index, not a unique constraint.
	RechargeUUID   string `bun:"recharge_uuid,notnull"`
	UserUUID       string `bun:"user_uuid,notnull"`
	AmountCents    int64  `bun:"amount_cents,notnull"`
	Currency       string `bun:"currency,notnull,default:'CNY'"`
	OrderNo        string `bun:"order_no,notnull"`
	RechargeTime   bun.NullTime
	PaymentChannel string `bun:"payment_channel,notnull,default:''"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AccountInvoiceRecharge{}); err != nil {
			return fmt.Errorf("create table account_invoice_recharges failed: %w", err)
		}

		if _, err := db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoiceRecharge{}).
			Index("idx_account_invoice_recharges_invoice_id").
			Column("invoice_id").
			Exec(ctx); err != nil {
			return fmt.Errorf("create index for AccountInvoiceRecharge failed: %w", err)
		}

		if _, err := db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoiceRecharge{}).
			Index("idx_account_invoice_recharges_user_uuid").
			Column("user_uuid").
			Exec(ctx); err != nil {
			return fmt.Errorf("create index for AccountInvoiceRecharge failed: %w", err)
		}

		if _, err := db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoiceRecharge{}).
			Index("idx_account_invoice_recharges_recharge_uuid").
			Column("recharge_uuid").
			Exec(ctx); err != nil {
			return fmt.Errorf("create index for AccountInvoiceRecharge failed: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, (*AccountInvoiceRecharge)(nil))
	})
}
