package migrations

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AccountEvent struct {
	EventUUID uuid.UUID         `bun:"type:uuid,notnull" json:"event_uuid"`
	EventBody map[string]string `bun:",hstore" json:"event_body"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountEvent{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountEvent{})
	})
}
