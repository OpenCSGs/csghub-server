package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type UserLike struct {
	ID           int64     `bun:",pk,autoincrement" json:"id"`
	UserID       int64     `bun:",notnull" json:"user_id"`
	RepoID       int64     `bun:",notnull" json:"repo_id"`
	CollectionID int64     `bun:",notnull" json:"collection_id"`
	DeletedAt    time.Time `bun:",soft_delete,nullzero"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, UserLike{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*UserLike)(nil)).
			Index("idx_user_likes_user_id").
			Column("user_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, UserLike{})
	})
}
