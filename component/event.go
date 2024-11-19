package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type eventComponentImpl struct {
	es database.EventStore
}

// NewEventComponent creates a new EventComponent

type EventComponent interface {
	NewEvents(ctx context.Context, events []types.Event) error
}

func NewEventComponent() EventComponent {
	return &eventComponentImpl{
		es: database.NewEventStore(),
	}
}

func (ec *eventComponentImpl) NewEvents(ctx context.Context, events []types.Event) error {
	var dbevents []database.Event
	for _, e := range events {
		dbevents = append(dbevents, database.Event{
			Module:    e.Module,
			EventID:   e.ID,
			Value:     e.Value,
			Extension: e.Extension,
		})
	}

	return ec.es.BatchSave(ctx, dbevents)
}
