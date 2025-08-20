package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type RepoRelation struct {
	ID         int64 `bun:",pk,autoincrement" json:"id"`
	FromRepoID int64 `bun:",notnull" json:"from_repo_id"`
	ToRepoID   int64 `bun:",notnull" json:"to_repo_id"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, RepoRelation{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*RepoRelation)(nil)).
			Index("idx_repo_relation_from_repo_id").
			Column("from_repo_id").
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*RepoRelation)(nil)).
			Index("idx_repo_relation_to_repo_id").
			Column("to_repo_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RepoRelation{})
	})
}
