package database

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/utils/payment/consts"
	"time"
)

type PaymentStore interface {
	CreatePayment(ctx context.Context, payment *Payment) error
	GetPaymentByID(ctx context.Context, paymentUUID string) (*Payment, error)
	GetPaymentByOrderNo(ctx context.Context, orderNo string) (*Payment, error)
	UpdatePayment(ctx context.Context, payment *Payment) error
	ListPayments(ctx context.Context, filter *PaymentFilter) ([]*Payment, error)
}

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
	Amount int64 `bun:",notnull,skipupdate" json:"amount"`

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

type PaymentStoreImpl struct {
	db *DB
}

func NewPaymentDBStoreWithDB(db *DB) PaymentStore {
	return &PaymentStoreImpl{db: db}
}

func NewPaymentStore() PaymentStore {
	return NewPaymentDBStoreWithDB(defaultDB)
}

func (ps *PaymentStoreImpl) CreatePayment(ctx context.Context, payment *Payment) error {
	_, err := ps.db.Operator.Core.NewInsert().
		Model(payment).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create payment record, error: %w", err)
	}
	return nil
}

func (ps *PaymentStoreImpl) GetPaymentByID(ctx context.Context, paymentUUID string) (*Payment, error) {
	var payment Payment
	err := ps.db.Operator.Core.NewSelect().
		Model(&payment).
		Where("uuid = ?", paymentUUID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment by uuid, error: %w", err)
	}
	return &payment, nil
}

func (ps *PaymentStoreImpl) GetPaymentByOrderNo(ctx context.Context, orderNo string) (*Payment, error) {
	var payment Payment
	err := ps.db.Operator.Core.NewSelect().
		Model(&payment).
		Where("order_no = ?", orderNo).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment by orderNo, error: %w", err)
	}
	return &payment, nil
}

func (ps *PaymentStoreImpl) UpdatePayment(ctx context.Context, payment *Payment) error {
	payment.UpdatedAt = time.Now()
	_, err := ps.db.Operator.Core.NewUpdate().
		Model(payment).
		Where("payment_uuid = ?", payment.PaymentUUID). // 根据 UUID 定位记录
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update payment record, error: %w", err)
	}
	return nil
}

func (ps *PaymentStoreImpl) ListPayments(ctx context.Context, filter *PaymentFilter) ([]*Payment, error) {
	var payments []*Payment
	query := ps.db.Operator.Core.NewSelect().
		Model(&payments).
		Order("created_at DESC")

	if filter != nil {
		if filter.UserUUID != "" {
			query.Where("user_uuid = ?", filter.UserUUID)
		}
		if filter.Paid != nil {
			query.Where("paid = ?", *filter.Paid)
		}
		if filter.Channel != nil {
			query.Where("channel = ?", *filter.Channel)
		}
		if filter.Limit > 0 {
			query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query.Offset(filter.Offset)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list payments, error: %w", err)
	}
	return payments, nil
}

type PaymentFilter struct {
	UserUUID string
	Paid     *bool
	Channel  *consts.PaymentChannel
	Limit    int
	Offset   int
}
