package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewCreateIndex().
			Model((*Metadata)(nil)).
			Index("idx_metadata_model_params_repo_id").
			Column("model_params", "repository_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_metadata_model_params_repo_id fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*RepositoryStatistics)(nil)).
			Index("idx_repository_statistics_total_size_repo_id_branch").
			Column("total_size", "repository_id", "branch").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_repository_statistics_total_size_repo_id_branch fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDropIndex().
			Model((*RepositoryStatistics)(nil)).
			Index("idx_repository_statistics_total_size_repo_id_branch").
			IfExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropIndex().
			Model((*Metadata)(nil)).
			Index("idx_metadata_model_params_repo_id").
			IfExists().
			Exec(ctx)
		return err
	})
}
