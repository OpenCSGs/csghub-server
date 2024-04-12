package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, database.RepoRelation{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*database.RepoRelation)(nil)).
			Index("idx_repo_relation_from_repo_id").
			Column("from_repo_id").
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*database.RepoRelation)(nil)).
			Index("idx_repo_relation_to_repo_id").
			Column("to_repo_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.RepoRelation{})
	})
}
