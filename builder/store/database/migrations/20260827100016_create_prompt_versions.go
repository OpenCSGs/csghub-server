package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type PromptVersion struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// PromptID identifies the Prompt repository that owns this version.
	PromptID int64 `bun:",notnull" json:"prompt_id"`
	// FilePath identifies the prompt file within the repository.
	FilePath string `bun:",notnull" json:"file_path"`
	// Version is the user-defined version name, for example v1.
	Version string `bun:",notnull" json:"version"`
	// Hash points to the latest Git commit saved for this editable version.
	Hash string `bun:",notnull" json:"hash"`
	// Changelog describes why the version was created.
	Changelog string `bun:",type:text" json:"changelog"`
	// times provides CreatedAt and UpdatedAt timestamps.
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, PromptVersion{}); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model((*PromptVersion)(nil)).
			Index("idx_unique_prompt_versions_prompt_id_file_path_version").
			Column("prompt_id", "file_path", "version").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create prompt version unique index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, PromptVersion{})
	})
}
