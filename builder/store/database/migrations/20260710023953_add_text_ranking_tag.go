package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		exists, err := db.NewSelect().Model((*Tag)(nil)).
			Where("name = ? AND category = ? AND scope = ?", "text-ranking", "task", "model").
			Exists(ctx)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		tag := Tag{
			Name:     "text-ranking",
			Category: "task",
			Group:    "natural_language_processing",
			Scope:    "model",
			ShowName: "文本排序",
			BuiltIn:  true,
		}
		_, err = db.NewInsert().Model(&tag).Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDelete().Model((*Tag)(nil)).
			Where("name = ? AND category = ? AND scope = ? AND built_in = true", "text-ranking", "task", "model").
			Exec(ctx)
		return err
	})
}
