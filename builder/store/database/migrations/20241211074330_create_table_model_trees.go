package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type ModelTree struct {
	ID           int64               `bun:",pk,autoincrement" json:"id"`
	SourceRepoID int64               `bun:",notnull" json:"source_repo_id"`
	SourcePath   string              `bun:",notnull" json:"source_path"`
	TargetRepoID int64               `bun:",notnull" json:"target_repo_id"`
	TargetPath   string              `bun:",notnull" json:"target_path"`
	Relation     types.ModelRelation `bun:",notnull" json:"relation"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, ModelTree{})
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE model_trees ADD CONSTRAINT chk_diff CHECK (source_repo_id <> target_repo_id)")
		if err != nil {
			return fmt.Errorf("failed to add chk_diff for model_trees table: %w", err)
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE model_trees ADD CONSTRAINT unique_model_tree_relation UNIQUE (source_repo_id, target_repo_id)")
		if err != nil {
			return fmt.Errorf("failed to add chk_diff for model_trees table: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*ModelTree)(nil)).
			Index("idx_model_tree_ids_relation").
			Column("source_repo_id", "target_repo_id", "relation").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ModelTree{})
	})
}
