package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountSubscription struct {
	ID              int64         `bun:",pk,autoincrement" json:"id"`
	UserUUID        string        `bun:",notnull" json:"user_uuid"`
	SkuType         types.SKUType `bun:",notnull" json:"sku_type"`
	PriceID         int64         `bun:",notnull" json:"price_id"`
	ResourceID      string        `bun:",notnull" json:"resource_id"`
	Status          string        `bun:",notnull" json:"status"`
	ActionUser      string        `bun:",notnull" json:"action_user"`
	StartAt         time.Time     `bun:",notnull" json:"start_at"`
	EndAt           time.Time     `bun:",nullzero" json:"end_at"`
	LastBillID      int64         `bun:",notnull,unique" json:"last_bill_id"`
	LastPeriodStart time.Time     `bun:",notnull" json:"last_period_start"`
	LastPeriodEnd   time.Time     `bun:",notnull" json:"last_period_end"`
	AmountPaidTotal float64       `bun:",notnull" json:"amount_paid_total"`
	AmountPaidCount int64         `bun:",notnull" json:"amount_paid_count"`
	NextPriceID     int64         `bun:",nullzero" json:"next_price_id"`
	NextResourceID  string        `bun:",nullzero" json:"next_resource_id"`
	times
}

type AccountSubscriptionBill struct {
	ID          int64     `bun:",pk,autoincrement" json:"id"`
	SubID       int64     `bun:",notnull" json:"sub_id"`
	EventUUID   string    `bun:",notnull,unique" json:"event_uuid"`
	UserUUID    string    `bun:",notnull" json:"user_uuid"`
	AmountPaid  float64   `bun:",notnull" json:"amount_paid"`
	Status      string    `bun:",notnull" json:"status"`
	Reason      string    `bun:",notnull" json:"reason"`
	PeriodStart time.Time `bun:",notnull" json:"period_start"`
	PeriodEnd   time.Time `bun:",notnull" json:"period_end"`
	PriceID     int64     `bun:",notnull" json:"price_id"`
	ResourceID  string    `bun:",notnull" json:"resource_id"`
	Explain     string    `bun:",nullzero" json:"explain"`
	times
}

type AccountSubscriptionUsage struct {
	ID           int64   `bun:",pk,autoincrement" json:"id"`
	UserUUID     string  `bun:",notnull" json:"user_uuid"`
	ResourceID   string  `bun:",notnull" json:"resource_id"`
	ResourceName string  `bun:",notnull" json:"resource_name"`
	CustomerID   string  `bun:",notnull" json:"customer_id"`
	Used         float64 `bun:",notnull" json:"used"`
	Quota        float64 `bun:",notnull" json:"quota"`
	BillID       int64   `bun:",nullzero" json:"bill_id"`    // for pro or team ver
	BillMonth    string  `bun:",nullzero" json:"bill_month"` // YYYY-MM format for free ver
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AccountSubscription{}, AccountSubscriptionBill{}, AccountSubscriptionUsage{}); err != nil {
			return fmt.Errorf("failed to create table subscription/subscription_bill/AccountSubscriptionUsage, error: %w", err)
		}

		_, err := db.NewCreateIndex().
			Model(&AccountSubscription{}).
			Index("idx_account_subscription_status_useruuid_startat_skutype").
			Column("status", "user_uuid", "start_at", "sku_type").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription on status/user_uuid/start_at/sku_type")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscription{}).
			Index("idx_account_subscription_startat_status_skutype").
			Column("start_at", "status", "sku_type").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription on start_at/status/sku_type")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscription{}).
			Index("idx_account_subscription_useruuid_skutype").
			Column("user_uuid", "sku_type").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription on user_uuid/sku_type")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscription{}).
			Index("idx_account_subscription_status_lastperiodend").
			Column("status", "last_period_end").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription on status/last_period_end")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscriptionBill{}).
			Index("idx_account_subscription_bill_subid_status").
			Column("sub_id", "status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription_bill on sub_id/status")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscriptionBill{}).
			Index("idx_account_subscription_bill_createdat_useruuid_status").
			Column("created_at", "user_uuid", "status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription_bill on created_at/user_uuid/status")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscriptionUsage{}).
			Index("idx_account_subscription_usage_billid_useruuid_resourceid_resourcename_customerid").
			Column("bill_id", "user_uuid", "resource_id", "resource_name", "customer_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription_usage on billid/useruuid/resourceid/resourcename/customerid")
		}

		_, err = db.NewCreateIndex().
			Model(&AccountSubscriptionUsage{}).
			Index("idx_account_subscription_usage_billmonth_useruuid_resourceid_resourcename_customerid").
			Column("bill_month", "user_uuid", "resource_id", "resource_name", "customer_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription_usage on bill_month/useruuid/resourceid/resourcename/customerid")
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountSubscription{}, AccountSubscriptionBill{}, AccountSubscriptionUsage{})
	})
}
