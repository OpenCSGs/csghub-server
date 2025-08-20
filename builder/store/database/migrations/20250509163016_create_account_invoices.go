package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// AccountInvoice represents an invoice record.
type AccountInvoice struct {
	ID            int       `bun:"id,pk,autoincrement"`
	UserUUID      string    `bun:"user_uuid,notnull" json:"user_uuid"`
	UserName      string    `bun:"user_name,notnull" json:"user_name"`              // User name
	TitleType     string    `bun:"title_type,notnull" json:"title_type"`            // Invoice title type
	InvoiceType   string    `bun:"invoice_type,notnull" json:"invoice_type"`        // Invoice type
	BillCycle     string    `bun:"bill_cycle,notnull" json:"bill_cycle"`            // Billing cycle
	InvoiceTitle  string    `bun:"invoice_title,notnull" json:"invoice_title"`      // Invoice title
	ApplyTime     time.Time `bun:"apply_time,type:timestamp" json:"apply_time"`     // Invoice application time
	InvoiceAmount float64   `bun:"invoice_amount,notnull" json:"invoice_amount"`    // Invoice amount
	Status        string    `bun:"status,notnull" json:"status"`                    // Invoice status
	Reason        string    `bun:"reason,notnull" json:"reason"`                    // Reason
	InvoiceDate   time.Time `bun:"invoice_date,type:timestamp" json:"invoice_date"` // Invoice issuance date
	InvoiceURL    string    `bun:"invoice_url,notnull" json:"invoice_url"`          // Invoice URL
	// Redundant fields for list display
	TaxpayerID     string `bun:"taxpayer_id,notnull" json:"taxpayer_id"`         // Taxpayer identification number
	BankName       string `bun:"bank_name,notnull" json:"bank_name"`             // Bank name
	BankAccount    string `bun:"bank_account,notnull" json:"bank_account"`       // Bank account number
	RegisteredAddr string `bun:"registered_addr,notnull" json:"registered_addr"` // Registered address
	ContactPhone   string `bun:"contact_phone,notnull" json:"contact_phone"`     // Contact phone number
	Email          string `bun:"email,notnull" json:"email"`                     // Email address

	times
}

// AccountInvoiceTitle represents an invoice title record.
type AccountInvoiceTitle struct {
	ID           int64  `bun:"id,pk,autoincrement"`
	UserUUID     string `bun:"user_uuid,notnull" json:"user_uuid"` // User
	UserName     string `bun:"user_name,notnull" json:"user_name"`
	Title        string `bun:"title,notnull" json:"title"` // Invoice title name
	TitleType    string `bun:",notnull" json:"title_type"` // Invoice title type
	InvoiceType  string `bun:"invoice_type,notnull" json:"invoice_type"`
	TaxID        string `bun:"tax_id,notnull" json:"tax_id"`       // Taxpayer identification number
	Address      string `bun:"address" json:"address"`             // Registered address
	BankName     string `bun:"bank_name" json:"bank_name"`         // Bank name
	BankAccount  string `bun:"bank_account" json:"bank_account"`   // Bank account number
	ContactPhone string `bun:"contact_phone" json:"contact_phone"` // Contact phone number
	Email        string `bun:"email" json:"email"`                 // Email address
	IsDefault    bool   `bun:"is_default" json:"is_default"`       // Whether it is the default title

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountInvoice{}, AccountInvoiceTitle{})
		if err != nil {
			return fmt.Errorf("create tables failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoice{}).
			Index("idx_account_invoice_user_uuid").
			Column("user_uuid").
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("create index for AccountInvoice failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoice{}).
			Index("idx_account_invoice_user_uuid_bill_cycle").
			Column("user_uuid", "bill_cycle").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index for AccountInvoice failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			IfNotExists().
			Model(&AccountInvoiceTitle{}).
			Index("idx_account_invoice_title_user_uuid").
			Column("user_uuid").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index for AccountInvoiceTitle failed: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, (*AccountInvoice)(nil), (*AccountInvoiceTitle)(nil))
	})

}
