package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Discussion{}, Comment{})
		if err != nil {
			return err
		}

		//create index for table discussions
		_, err = db.NewCreateIndex().Model(&Discussion{}).
			Column("discussionable_type", "discussionable_id").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for table discussion: %w", err)
		}
		//create index for table comments
		_, err = db.NewCreateIndex().Model(&Comment{}).
			Column("commentable_type", "commentable_id").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for table comment: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Discussion{}, Comment{})
	})
}

type Discussion struct {
	ID                 int64     `bun:"id,pk,autoincrement"`
	UserID             int64     `bun:"user_id,notnull"`
	Title              string    `bun:"title,notnull"`
	DiscussionableID   int64     `bun:"discussionable_id,notnull"`
	DiscussionableType string    `bun:"discussionable_type,notnull"`
	CommentCount       int64     `bun:"comment_count,notnull,default:0"`
	CreatedAt          time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt          time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

type Comment struct {
	ID              int64     `bun:"id,pk,autoincrement"`
	Content         string    `bun:"content"`
	CommentableType string    `bun:"commentable_type,notnull"`
	CommentableID   int64     `bun:"commentable_id,notnull"`
	UserID          int64     `bun:"user_id,notnull"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}
