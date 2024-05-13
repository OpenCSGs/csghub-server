package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, database.Mirror{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*database.Mirror)(nil)).
			Index("idx_mirrors_repository_id").
			Column("repository_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.Mirror{})
	})

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, database.MirrorSource{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.MirrorSource{})
	})
}
