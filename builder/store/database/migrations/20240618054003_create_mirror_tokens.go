package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type MirrorToken struct {
	ID              int64  `bun:",pk,autoincrement" json:"id"`
	Token           string `bun:",notnull" json:"token"`
	ConcurrentCount int    `bun:",nullzero" json:"concurrent_count"`
	MaxBandwidth    int    `bun:",nullzero" json:"max_bandwidth"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, MirrorToken{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, MirrorToken{})
	})
}
