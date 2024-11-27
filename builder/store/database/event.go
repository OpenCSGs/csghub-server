package database

import "context"

type eventStoreImpl struct {
	db *DB
}

type EventStore interface {
	Save(ctx context.Context, event Event) error
	// batch insert
	BatchSave(ctx context.Context, events []Event) error
}

func NewEventStore() EventStore {
	return &eventStoreImpl{
		db: defaultDB,
	}
}

func NewEventStoreWithDB(db *DB) EventStore {
	return &eventStoreImpl{
		db: db,
	}
}

func (s *eventStoreImpl) Save(ctx context.Context, event Event) error {
	return assertAffectedOneRow(s.db.Core.NewInsert().Model(&event).Exec(ctx))
}

// batch insert
func (s *eventStoreImpl) BatchSave(ctx context.Context, events []Event) error {
	result, err := s.db.Core.NewInsert().Model(&events).Exec(ctx)
	return assertAffectedXRows(int64(len(events)), result, err)
}
