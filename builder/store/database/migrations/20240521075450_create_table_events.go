package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, database.Event{}); err != nil {
			return err
		}

		_, err := db.NewCreateIndex().Model(&database.Event{}).
			Index("idx_events_created_at").
			Column("created_at").
			Exec(ctx)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.Event{})
	})
}
