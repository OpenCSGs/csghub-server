package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/utils/payment/consts"
	"time"
)

type Payment struct {
	bun.BaseModel `bun:"table:payment_payment"`

	PaymentUUID string `bun:",notnull,pk,skipupdate" json:"payment_uuid"`

	// Transaction serial number returned by the payment channel.
	TransactionNo string `json:"transaction_no"`

	// Order number, tailored to the requirements of each channel, and must be unique within the business system.
	// For example, in the case of a recharge, this field corresponds to the orderNo in the recharge table.
	// For payment channels, this parameter typically corresponds to out_trade_no.
	OrderNo string `bun:",notnull,skipupdate" json:"order_no"`

	// Payment channel.
	Channel consts.PaymentChannel `bun:",notnull,skipupdate" json:"channel"`

	// Transformed into a QR code for frontend scanning payment scenarios.
	CodeUrl string `bun:",skipupdate" json:"code_url"`

	// Payment credentials used by the client to initiate a payment.
	Credentials json.RawMessage `bun:",nullzero" json:"credentials"`

	// Client IP address.
	ClientIp string `bun:",skipupdate" json:"client_ip"`

	// Total amount in the smallest currency unit (e.g., in CNY, this is expressed in cents).
	Amount float64 `bun:",notnull,skipupdate" json:"amount"`

	// 3-letter ISO currency code, represented in uppercase letters.
	Currency string `bun:",notnull,skipupdate,default:'CNY'" json:"currency"`

	// Product title, limited to a maximum of 32 Unicode characters.
	Subject string `bun:",notnull,skipupdate" json:"subject"`

	// Product description, limited to a maximum of 128 Unicode characters.
	// Note: yeepay_wap restricts this parameter to a maximum of 100 Unicode characters;
	// some channels of Alipay do not support special characters.
	Body string `bun:",skipupdate" json:"body"`

	// Custom fields for business-specific use cases.
	Extra string `bun:",skipupdate" json:"extra"`

	// Indicates whether the payment has been completed.
	Paid bool `json:"paid"`

	// Indicates whether the order has been revoked.
	Reversed bool `json:"reversed"`

	// Unix timestamp representing the time when the payment was completed.
	TimePaid time.Time `bun:",nullzero" json:"time_paid"`

	// Unix timestamp representing the expiration time of the order.
	TimeExpire time.Time `bun:",nullzero" json:"time_expire"`

	// Payment creation time.
	CreatedAt time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`

	// Payment update time.
	UpdatedAt time.Time `bun:",notnull,default:current_timestamp" json:"updated_at"`

	// Error code returned in case of payment failure.
	FailureCode string `json:"failure_code"`

	// Error message or description for the payment failure.
	FailureMsg string `json:"failure_msg"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Payment{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*Payment)(nil)).
			Index("idx_payment_order_no").
			Column("order_no").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_payment_order_no fail: %w", err)
		}
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Payment{})
	})
}
