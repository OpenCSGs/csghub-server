package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// First, check if skill scope tag_category exists
		skillCategoryExists, err := db.NewSelect().
			TableExpr("tag_categories").
			Where("name = ? AND scope = ?", "task", "skill").
			Limit(1).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("checking skill scope tag_category existence: %w", err)
		}

		if !skillCategoryExists {
			// Create skill scope tag_category
			tagCategory := &TagCategory{
				Name:     "task",
				Scope:    "skill",
				ShowName: "任务",
				Enabled:  true,
			}
			_, err = db.NewInsert().Model(tagCategory).Exec(ctx)
			if err != nil {
				return fmt.Errorf("creating skill scope tag_category: %w", err)
			}

			fmt.Println("Created skill scope tag_category (task)")
		}

		// Check if skill scope agentichub-skills tag exists
		skillTagExists, err := db.NewSelect().
			TableExpr("tags").
			Where("name = ? AND scope = ?", "agentichub-skills", "skill").
			Limit(1).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("checking skill scope tag existence: %w", err)
		}

		if !skillTagExists {
			// Skill scope tag doesn't exist, update code scope tag to skill scope
			result, err := db.NewUpdate().
				TableExpr("tags").
				Where("name = ? AND scope = ?", "agentichub-skills", "code").
				Set("scope = ?", "skill").
				Set("i18n_key = ?", "agentichub-skills").
				Set("updated_at = CURRENT_TIMESTAMP").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("updating code scope tag to skill scope: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			fmt.Printf("Updated %d agentichub-skills tags from code scope to skill scope\n", rowsAffected)
		} else {
			// Skill scope tag already exists, migrate repository_tags associations
			// Get the code scope tag ID
			var codeTagID int64
			err := db.NewSelect().
				TableExpr("tags").
				Column("id").
				Where("name = ? AND scope = ?", "agentichub-skills", "code").
				Scan(ctx, &codeTagID)
			if err != nil {
				// Code scope tag doesn't exist, nothing to do
				fmt.Println("No code scope agentichub-skills tag found, nothing to migrate")
				return nil
			}

			// Get the skill scope tag ID
			var skillTagID int64
			err = db.NewSelect().
				TableExpr("tags").
				Column("id").
				Where("name = ? AND scope = ?", "agentichub-skills", "skill").
				Scan(ctx, &skillTagID)
			if err != nil {
				return fmt.Errorf("getting skill scope tag ID: %w", err)
			}

			// Update repository_tags: change tag_id from code tag to skill tag
			result, err := db.NewUpdate().
				TableExpr("repository_tags").
				Where("tag_id = ?", codeTagID).
				Set("tag_id = ?", skillTagID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("updating repository_tags associations: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			fmt.Printf("Migrated %d repository_tags associations from code scope to skill scope tag\n", rowsAffected)

			// Delete the old code scope tag
			_, err = db.NewDelete().
				TableExpr("tags").
				Where("id = ?", codeTagID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("deleting old code scope tag: %w", err)
			}

			fmt.Println("Deleted old code scope agentichub-skills tag")
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration: reverse the changes
		// Delete skill scope tag_category if it exists
		_, err := db.NewDelete().
			TableExpr("tag_categories").
			Where("name = ? AND scope = ?", "task", "skill").
			Exec(ctx)
		if err != nil {
			fmt.Printf("Warning: failed to delete skill scope tag_category: %v\n", err)
		} else {
			fmt.Println("Deleted skill scope tag_category (task)")
		}

		// Check if code scope agentichub-skills tag exists
		codeTagExists, err := db.NewSelect().
			TableExpr("tags").
			Where("name = ? AND scope = ?", "agentichub-skills", "code").
			Limit(1).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("checking code scope tag existence: %w", err)
		}

		if !codeTagExists {
			// Code scope tag doesn't exist, update skill scope tag back to code scope
			result, err := db.NewUpdate().
				TableExpr("tags").
				Where("name = ? AND scope = ?", "agentichub-skills", "skill").
				Set("scope = ?", "code").
				Set("updated_at = CURRENT_TIMESTAMP").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("updating skill scope tag back to code scope: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			fmt.Printf("Reverted %d agentichub-skills tags from skill scope back to code scope\n", rowsAffected)
		}

		fmt.Print(" [down migration] ")
		return nil
	})
}
