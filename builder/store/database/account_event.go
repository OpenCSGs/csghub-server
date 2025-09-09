package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type accountEventStoreImpl struct {
	db *DB
}

type AccountEventStore interface {
	GetByEventID(ctx context.Context, eventID uuid.UUID) (*AccountEvent, error)
	Create(ctx context.Context, input AccountEvent) error
}

func NewAccountEventStore() AccountEventStore {
	return &accountEventStoreImpl{
		db: defaultDB,
	}
}

func NewAccountEventStoreWithDB(db *DB) AccountEventStore {
	return &accountEventStoreImpl{
		db: db,
	}
}

type AccountEvent struct {
	EventUUID  uuid.UUID         `bun:"type:uuid,notnull" json:"event_uuid"`
	EventBody  map[string]string `bun:",hstore" json:"event_body"`
	Duplicated bool              `json:"duplicated"`
	CreatedAt  time.Time         `bun:",notnull,default:current_timestamp" json:"created_at"`
}

func (s *accountEventStoreImpl) GetByEventID(ctx context.Context, eventID uuid.UUID) (*AccountEvent, error) {
	event := &AccountEvent{}
	err := s.db.Core.NewSelect().Model(event).Where("event_uuid = ?", eventID).Scan(ctx, event)
	return event, err
}

func (s *accountEventStoreImpl) Create(ctx context.Context, input AccountEvent) error {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("insert event log failed, error:%w", err)
	}
	return nil
}
