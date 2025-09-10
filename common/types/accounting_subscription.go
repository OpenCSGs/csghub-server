package types

import (
	"time"

	"github.com/google/uuid"
)

var AccountingSubscriptionQueue = "accounting_subscription_queue"

var (
	SubscriptionFree = "free"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusClosed   SubscriptionStatus = "closed"
)

type BillingReasion string

const (
	BillingReasonSubscriptionCreate     BillingReasion = "subscription_create"
	BillingReasonSubscriptionCycle      BillingReasion = "subscription_cycle"
	BillingReasionSubscriptionUpgrade   BillingReasion = "subscription_upgrade"
	BillingReasionSubscriptionDowngrade BillingReasion = "subscription_downgrade"
	BillingReasionSubscriptionClose     BillingReasion = "subscription_close"
	BillingReasonBalanceInsufficient    BillingReasion = "balance_insufficient"
	BillingReasonLostPrice              BillingReasion = "lost_price"
)

type BillingStatus string

const (
	BillingStatusPaid   BillingStatus = "paid"
	BillingStatusFailed BillingStatus = "unpaid"
)

type SubscriptionCreateReq struct {
	CurrentUser string    `json:"-"`
	UserUUID    string    `json:"-"`
	SkuType     SKUType   `json:"sku_type" binding:"required"`
	ResourceID  string    `json:"resource_id" binding:"required"`
	SkuUnitType string    `json:"sku_unit_type" binding:"required"`
	EventUUID   uuid.UUID `json:"-"`
}

type SubscriptionResp struct {
	ID              int64              `json:"id"`
	UserUUID        string             `bun:",notnull" json:"user_uuid"`
	SkuType         SKUType            `bun:",notnull" json:"sku_type"`
	PriceID         int64              `bun:",notnull" json:"price_id"`
	ResourceID      string             `bun:",notnull" json:"resource_id"`
	Status          SubscriptionStatus `bun:",notnull" json:"status"`
	ActionUser      string             `bun:",notnull" json:"action_user"`
	StartAt         time.Time          `bun:",notnull" json:"start_at"`
	EndAt           time.Time          `bun:",nullzero" json:"end_at"`
	LastBillID      int64              `bun:",notnull,unique" json:"last_bill_id"`
	LastPeriodStart time.Time          `bun:",notnull" json:"last_period_start"`
	LastPeriodEnd   time.Time          `bun:",notnull" json:"last_period_end"`
	AmountPaidTotal float64            `bun:",notnull" json:"amount_paid_total"`
	AmountPaidCount int64              `bun:",notnull" json:"amount_paid_count"`
	NextPriceID     int64              `bun:",nullzero" json:"next_price_id"`
	NextResourceID  string             `bun:",nullzero" json:"next_resource_id"`
	CreatedAt       time.Time          `bun:",nullzero" json:"created_at"`
}

type SubscriptionListReq struct {
	CurrentUser   string `json:"-"`
	UserUUID      string `json:"-"`
	Status        string `json:"-"`
	SkuType       int    `json:"-"`
	StartTime     string `json:"-"`
	EndTime       string `json:"-"`
	Per           int    `json:"-"`
	Page          int    `json:"-"`
	QueryUserUUID string `json:"-"`
}

type SubscriptionAllRes struct {
	Data            []SubscriptionResp `json:"data"`
	Total           int                `json:"total"`
	PaidTotalAmount float64            `json:"paid_total_amount"`
	PaidTotalCount  int                `json:"paid_total_count"`
}

type SubscriptionBillListReq struct {
	CurrentUser   string `json:"-"`
	UserUUID      string `json:"-"`
	QueryUserUUID string `json:"-"`
	Status        string `json:"-"`
	StartTime     string `json:"-"`
	EndTime       string `json:"-"`
	Per           int    `json:"-"`
	Page          int    `json:"-"`
}

type SubscriptionBillResp struct {
	ID          int64          `json:"id"`
	SubID       int64          `json:"sub_id"`
	EventUUID   string         `json:"event_uuid"`
	UserUUID    string         `json:"user_uuid"`
	AmountPaid  float64        `json:"amount_paid"`
	Status      BillingStatus  `json:"status"`
	Reason      BillingReasion `json:"reason"`
	PeriodStart time.Time      `json:"period_start"`
	PeriodEnd   time.Time      `json:"period_end"`
	PriceID     int64          `json:"price_id"`
	ResourceID  string         `json:"resource_id"`
	Explain     string         `json:"explain"`
	CreatedAt   time.Time      `json:"created_at"`
}

type SubscriptionBillAllRes struct {
	TotalAmount float64                `json:"total_amount"`
	Total       int                    `json:"total"`
	Data        []SubscriptionBillResp `json:"data"`
}

type SubscriptionGetReq struct {
	CurrentUser string    `json:"-"`
	UserUUID    string    `json:"-"`
	SubID       int64     `json:"-"`
	EventUUID   uuid.UUID `json:"-"`
	SkuType     int       `json:"-"`
}

type SubscriptionUpdateReq struct {
	CurrentUser string    `json:"-"`
	UserUUID    string    `json:"-"`
	SubID       int64     `json:"-"`
	SkuType     int       `json:"sku_type" binding:"required"`
	ResourceID  string    `json:"resource_id" binding:"required"`
	SkuUnitType string    `json:"sku_unit_type" binding:"required"`
	EventUUID   uuid.UUID `json:"-"`
}

type SubscriptionStatusResp struct {
	UserUUID       string              `json:"user_uuid"`
	SkuType        SKUType             `json:"sku_type"`
	SubID          int64               `json:"sub_id"`
	Status         string              `json:"status"`
	ResourceID     string              `json:"resource_id"`
	PeriodStart    int64               `json:"period_start"`
	PeriodEnd      int64               `json:"period_end"`
	BillID         int64               `json:"bill_id"`
	BillMonth      string              `json:"bill_month"`
	NextPriceID    int64               `json:"next_price_id"`
	NextResourceID string              `json:"next_resource_id"`
	Usage          []SubscriptionUsage `json:"usage"`
}

type SubscriptionUsage struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	CustomerID   string  `json:"customer_id"`
	Used         float64 `json:"used"`
	Quota        float64 `json:"quota"`
}

type SubscriptionBatchStatusReq struct {
	CurrentUser    string   `json:"-"`
	UserUUID       string   `json:"-"`
	SkuType        SKUType  `json:"sku_type" binding:"required"`
	QueryUserUUIDs []string `json:"query_user_uuids" binding:"required"`
}

type SubscriptionEventDetail struct {
	SkuType       SKUType `json:"sku_type"`
	PreResourceID string  `json:"pre_resource_id"`
	ResourceID    string  `json:"resource_id"`
	BillID        int64   `json:"bill_id"`
	BillMonth     string  `json:"bill_month"`
	PeriodStart   int64   `json:"period_start"`
	PeriodEnd     int64   `json:"period_end"`
}

type SubscriptionEvent struct {
	Uuid         string                  `json:"uuid"`
	UserUUID     string                  `json:"user_uuid"`
	CreatedAt    time.Time               `json:"created_at"`
	ReasonCode   int                     `json:"reason_code"`
	ReasonMsg    string                  `json:"reason_msg"`
	Subscription SubscriptionEventDetail `json:"subscription"`
}
