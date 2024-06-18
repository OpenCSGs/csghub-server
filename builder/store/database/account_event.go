package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type AccountEventStore struct {
	db *DB
}

func NewAccountEventStore() *AccountEventStore {
	return &AccountEventStore{
		db: defaultDB,
	}
}

type AccountEvent struct {
	EventUUID uuid.UUID         `bun:"type:uuid,notnull" json:"event_uuid"`
	EventBody map[string]string `bun:",hstore" json:"event_body"`
}

func (s *AccountEventStore) GetByEventID(ctx context.Context, eventID uuid.UUID) (*AccountEvent, error) {
	event := &AccountEvent{}
	err := s.db.Core.NewSelect().Model(event).Where("event_uuid = ?", eventID).Scan(ctx, event)
	return event, err
}

func (s *AccountEventStore) Create(ctx context.Context, input AccountEvent) error {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("insert event log failed, error:%w", err)
	}
	return nil
}
