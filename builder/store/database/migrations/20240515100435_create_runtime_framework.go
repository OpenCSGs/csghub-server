package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type RuntimeFramework struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	FrameName    string `bun:",notnull" json:"frame_name"`
	FrameVersion string `bun:",notnull" json:"frame_version"`
	FrameImage   string `bun:",nullzero" json:"frame_image"`
	Enabled      int64  `bun:",notnull" json:"enabled"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, RuntimeFramework{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RuntimeFramework{})
	})
}
