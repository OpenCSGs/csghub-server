package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// RepositoryStatistics represents the repository_statistics table
type RepositoryStatistics struct {
	bun.BaseModel `bun:"table:repository_statistics"`
	ID            int64 `bun:",pk,autoincrement"`
	RepositoryID  int64 `bun:",notnull,unique"`
	TotalSize     int64 `bun:",notnull,default:0"`
	NonLfsSize    int64 `bun:",notnull,default:0"`
	LfsSize       int64 `bun:",notnull,default:0"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Create repository_statistics table
		err := createTables(ctx, db, &RepositoryStatistics{})
		if err != nil {
			return err
		}

		// Create index for repository_id
		_, err = db.NewCreateIndex().Model(&RepositoryStatistics{}).
			Index("idx_repository_statistics_repository_id").
			Column("repository_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Drop repository_statistics table
		return dropTables(ctx, db, &RepositoryStatistics{})
	})
}
