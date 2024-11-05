package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type Prompt struct {
	ID           int64 `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64 `bun:",notnull" json:"repository_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Prompt{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*Prompt)(nil)).
			Index("idx_prompts_repositoryid").
			Column("repository_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Prompt{})
	})
}
