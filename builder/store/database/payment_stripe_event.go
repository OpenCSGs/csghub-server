package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type PaymentStripeEventStore interface {
	Create(ctx context.Context, event PaymentStripeEvent) (*PaymentStripeEvent, error)
	Get(ctx context.Context, eventID string) (*PaymentStripeEvent, error)
}

type PaymentStripeEvent struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	EventID   string `bun:",notnull,unique" json:"event_id"`
	EventType string `bun:",notnull" json:"event_type"`
	times
	EventBody string `bun:",notnull" json:"event_body"`
}

type paymentStripeEventStoreImpl struct {
	db *DB
}

func NewPaymentStripeEventStore() PaymentStripeEventStore {
	return &paymentStripeEventStoreImpl{
		db: defaultDB,
	}
}

func NewPaymentStripeEventStoreWithDB(db *DB) PaymentStripeEventStore {
	return &paymentStripeEventStoreImpl{
		db: db,
	}
}

func (s *paymentStripeEventStoreImpl) Create(ctx context.Context, event PaymentStripeEvent) (*PaymentStripeEvent, error) {
	res, err := s.db.Core.NewInsert().Model(&event).Exec(ctx, &event)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &event, nil
}

func (s *paymentStripeEventStoreImpl) Get(ctx context.Context, eventID string) (*PaymentStripeEvent, error) {
	var event PaymentStripeEvent
	err := s.db.Core.NewSelect().Model(&event).Where("event_id = ?", eventID).Scan(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &event, nil
}
