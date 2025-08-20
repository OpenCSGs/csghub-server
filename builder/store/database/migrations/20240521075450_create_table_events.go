package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type Event struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	Module    string    `bun:",notnull" json:"module"`
	EventID   string    `bun:",notnull" json:"event_id"`
	Value     string    `bun:",notnull" json:"value"`
	ClientID  string    `bun:"," json:"client_id"`
	ClientIP  string    `bun:"," json:"client_ip"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	Extension string    `bun:"," json:"extension"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, Event{}); err != nil {
			return err
		}

		_, err := db.NewCreateIndex().Model(&Event{}).
			Index("idx_events_created_at").
			Column("created_at").
			Exec(ctx)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Event{})
	})
}
