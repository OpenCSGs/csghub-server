package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type PaymentStripeEvent struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	EventID   string `bun:",notnull,unique" json:"event_id"`
	EventType string `bun:",notnull" json:"event_type"`
	times
	EventBody string `bun:",notnull" json:"event_body"`
}

type PaymentStripe struct {
	ID                 int64     `bun:",pk,autoincrement" json:"id"`
	ClientReferenceID  string    `bun:",notnull,unique" json:"client_reference_id"`
	UserUUID           string    `bun:",notnull" json:"user_uuid"`
	AmountTotal        int64     `bun:",notnull" json:"amount_total"`
	Currency           string    `bun:",notnull" json:"currency"`
	SessionID          string    `bun:",notnull,unique" json:"session_id"`
	SessionStatus      string    `bun:",nullzero" json:"session_status"`
	PaymentStatus      string    `bun:",nullzero" json:"payment_status"`
	SessionCreatedAt   time.Time `bun:",nullzero" json:"session_created_at"`
	SessionCompletedAt time.Time `bun:",nullzero" json:"session_completed_at"`
	SessionExpiresAt   time.Time `bun:",nullzero" json:"session_expires_at"`
	CustomerEmail      string    `bun:",nullzero" json:"customer_email"`
	CustomerName       string    `bun:",nullzero" json:"customer_name"`
	Mode               string    `bun:",nullzero" json:"mode"`
	LiveMode           bool      `bun:",nullzero" json:"live_mode"`
	PaymentIntentID    string    `bun:",nullzero" json:"payment_intent_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, PaymentStripeEvent{}, PaymentStripe{}); err != nil {
			return fmt.Errorf("failed to create table stripe_event and payment_stripe, error: %w", err)
		}

		_, err := db.NewCreateIndex().
			Model(&PaymentStripe{}).
			Index("idx_payment_stripe_createdat_useruuid_status").
			Column("session_created_at", "user_uuid", "session_status", "payment_status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for payment_stripe on session_created_at/user_uuid/session_status/payment_status")
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, PaymentStripeEvent{}, PaymentStripe{})
	})
}
