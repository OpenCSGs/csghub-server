package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AccountSyncQuota struct {
	UserID         int64 `bun:",pk" json:"user_id"`
	RepoCountLimit int64 `bun:",notnull" json:"repo_count_limit"`
	RepoCountUsed  int64 `bun:",notnull" json:"repo_count_used"`
	SpeedLimit     int64 `bun:",notnull" json:"speed_limit"`
	TrafficLimit   int64 `bun:",notnull" json:"traffic_limit"`
	TrafficUsed    int64 `bun:",notnull" json:"traffic_used"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountSyncQuota{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountSyncQuota{})
	})
}
