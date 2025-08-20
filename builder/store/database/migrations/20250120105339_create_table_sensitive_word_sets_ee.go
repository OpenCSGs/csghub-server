package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	type SensitiveWordSetCategory struct {
		ID       int64  `bun:",pk,autoincrement" json:"id"`
		Name     string `bun:",notnull" json:"name"`
		ShowName string `bun:",notnull" json:"show_name"`
	}

	type SensitiveWordSet struct {
		ID         int64  `bun:",pk,autoincrement" json:"id"`
		Name       string `bun:",notnull" json:"name"`
		ShowName   string `bun:",notnull" json:"show_name"`
		WordList   string `bun:",notnull" json:"word_list"`
		Enabled    bool   `bun:",notnull" json:"enabled"`
		CategoryID int64  `bun:"category_id,notnull" json:"category_id"`
		// many to one relation
		Category *SensitiveWordSetCategory `bun:"rel:belongs-to,join:category_id=id" json:"category"`

		times
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, SensitiveWordSetCategory{}, SensitiveWordSet{})
		if err != nil {
			return fmt.Errorf("failed to create table sensitive_word_sets tables: %w", err)
		}

		err = initSensitiveWordSetCategory(ctx, db)
		if err != nil {
			return fmt.Errorf("failed to init sensitive word set category: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SensitiveWordSet{}, SensitiveWordSetCategory{})
	})
}

func initSensitiveWordSetCategory(ctx context.Context, db *bun.DB) error {
	type SensitiveWordSetCategory struct {
		ID       int64  `bun:",pk,autoincrement" json:"id"`
		Name     string `bun:",notnull" json:"name"`
		ShowName string `bun:",notnull" json:"show_name"`
	}
	//init sensitive word set category
	var categories = []SensitiveWordSetCategory{
		{
			Name:     "politic",
			ShowName: "政治敏感",
		},
		{
			Name:     "porn",
			ShowName: "色情敏感",
		},
		{
			Name:     "violence",
			ShowName: "暴力",
		},
	}

	_, err := db.NewInsert().Model(&categories).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert categories to db: %w", err)
	}
	return nil
}
