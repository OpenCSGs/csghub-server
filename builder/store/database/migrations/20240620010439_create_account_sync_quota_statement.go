package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type AccountSyncQuotaStatement struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	UserID    int64     `bun:",notnull" json:"user_id"`
	RepoPath  string    `bun:",notnull" json:"repo_path"`
	RepoType  string    `bun:",notnull" json:"repo_type"`
	CreatedAt time.Time `bun:",notnull,default:current_timestamp" json:"created_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountSyncQuotaStatement{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountSyncQuotaStatement{})
	})
}
