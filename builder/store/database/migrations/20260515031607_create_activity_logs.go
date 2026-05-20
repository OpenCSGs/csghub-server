package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type ActivityLog struct {
	ID            int64     `bun:",pk,autoincrement"`
	UserUUID      string    `bun:",notnull"`
	Username      string    `bun:",notnull"`
	AuthType      string    `bun:",notnull"`
	Action        string    `bun:",notnull"`
	ResourceType  string    `bun:",notnull"`
	ResourceID    int64     `bun:",notnull"`
	ResourceName  string    `bun:",notnull"`
	IPAddress     string    `bun:",notnull"`
	UserAgent     string    `bun:","`
	OperationTime time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, ActivityLog{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ActivityLog{})
	})
}
