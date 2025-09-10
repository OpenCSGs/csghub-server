package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type UserTag struct {
	ID     int64 `bun:"column:pk,autoincrement"`
	UserID int64 `bun:",notnull"`
	TagID  int64 `bun:",notnull"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, UserTag{}); err != nil {
			return fmt.Errorf("failed to create table user_tag, error: %w", err)
		}

		_, err := db.NewCreateIndex().
			Model(&UserTag{}).
			Index("idx_user_id_link_tag_id").
			Column("user_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_subscription on status/user_uuid/start_at/sku_type")
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if err := dropTables(ctx, db, UserTag{}); err != nil {
			return fmt.Errorf("failed to drop table user_tag, error: %w", err)
		}
		return nil
	})
}
