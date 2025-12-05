package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// delete comments that have been deleted since we had switched to use force delete
		_, err := db.NewDelete().
			Table("comments").
			Where("deleted_at IS NOT NULL").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete comments: %w", err)
		}

		// delete discussions that have been deleted since we had switched to use force delete
		_, err = db.NewDelete().
			Table("discussions").
			Where("deleted_at IS NOT NULL").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete discussions: %w", err)
		}

		// get all discussion IDs
		discussionIDs := []int64{}
		err = db.NewSelect().
			Table("discussions").
			Column("id").
			Scan(ctx, &discussionIDs)
		if err != nil {
			return fmt.Errorf("failed to get discussion IDs: %w", err)
		}

		// update the comment_count for discussion one by one
		for _, discussionID := range discussionIDs {
			_, err = db.NewUpdate().
				Table("discussions").
				Set("comment_count = (SELECT COUNT(*) FROM comments WHERE commentable_type = 'discussion' AND commentable_id = ?)", discussionID).
				Where("id = ?", discussionID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to update discussion comment count for discussion ID %d: %w", discussionID, err)
			}
		}

		// create unique index on commentable_type, commentable_id, and created_at for comments table
		_, err = db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_commentable_type_commentable_id_created_at ON comments (commentable_type, commentable_id, created_at)")
		if err != nil {
			return fmt.Errorf("failed to create index on commentable_type, commentable_id, and created_at: %w", err)
		}
		return nil

	}, func(ctx context.Context, db *bun.DB) error {
		// drop index on commentable_type, commentable_id, and created_at for comments table
		_, err := db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_commentable_type_commentable_id_created_at")
		if err != nil {
			return fmt.Errorf("failed to drop index idx_commentable_type_commentable_id_created_at for comments table: %w", err)
		}
		return nil
	})
}
