package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type MirrorTask struct {
	ID                 int64  `bun:",pk,autoincrement"`
	MirrorID           int64  `bun:",notnull"`
	ErrorMessage       string `bun:",nullzero"`
	Status             string `bun:",notnull"`
	RetryCount         int    `bun:",notnull,default:0"`
	Payload            string `bun:","`
	Priority           int    `bun:",notnull"`
	Progress           int    `bun:",notnull,default:0"`
	BeforeLastCommitID string `bun:","`
	AfterLastCommitID  string `bun:","`

	StartedAt  time.Time `bun:",nullzero"`
	FinishedAt time.Time `bun:",nullzero"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, &MirrorTask{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &MirrorTask{})
	})
}
