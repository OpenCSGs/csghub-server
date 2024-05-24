package database

import "context"

type EventStore struct {
	db *DB
}

func NewEventStore() *EventStore {
	return &EventStore{
		db: defaultDB,
	}
}

func (s *EventStore) Save(ctx context.Context, event Event) error {
	return assertAffectedOneRow(s.db.Core.NewInsert().Model(&event).Exec(ctx))
}

// batch insert
func (s *EventStore) BatchSave(ctx context.Context, events []Event) error {
	result, err := s.db.Core.NewInsert().Model(&events).Exec(ctx)
	return assertAffectedXRows(int64(len(events)), result, err)
}
