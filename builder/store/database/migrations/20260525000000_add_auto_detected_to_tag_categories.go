package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Add auto_detected column to tag_categories table
		_, err := db.ExecContext(ctx,
			`ALTER TABLE tag_categories ADD COLUMN IF NOT EXISTS auto_detected BOOLEAN NOT NULL DEFAULT FALSE`)
		if err != nil {
			return fmt.Errorf("failed to add auto_detected column: %w", err)
		}
		fmt.Println("Added auto_detected column to tag_categories table")

		// Set auto_detected = true for categories that are system-managed
		// framework (model scope): detected by file extensions (.safetensors, .pt, .onnx, etc.)
		_, err = db.NewUpdate().
			TableExpr("tag_categories").
			Where("name = ? AND scope = ?", "framework", "model").
			Set("auto_detected = ?", true).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update framework category: %w", err)
		}

		// runtime_framework (model scope): detected by model architecture in config.json
		_, err = db.NewUpdate().
			TableExpr("tag_categories").
			Where("name = ? AND scope = ?", "runtime_framework", "model").
			Set("auto_detected = ?", true).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update runtime_framework category: %w", err)
		}

		// evaluation (dataset scope): detected by tag_rules table mapping
		_, err = db.NewUpdate().
			TableExpr("tag_categories").
			Where("name = ? AND scope = ?", "evaluation", "dataset").
			Set("auto_detected = ?", true).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update evaluation category: %w", err)
		}

		fmt.Println("Set auto_detected=true for framework(model), runtime_framework(model), evaluation(dataset)")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration: remove the auto_detected column
		_, err := db.ExecContext(ctx,
			`ALTER TABLE tag_categories DROP COLUMN IF EXISTS auto_detected`)
		if err != nil {
			return fmt.Errorf("failed to drop auto_detected column: %w", err)
		}
		fmt.Println(" [down migration] Removed auto_detected column from tag_categories")
		return nil
	})
}
