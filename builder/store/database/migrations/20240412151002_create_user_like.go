package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, database.UserLike{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*database.UserLike)(nil)).
			Index("idx_user_likes_user_id").
			Column("user_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.UserLike{})
	})
}
