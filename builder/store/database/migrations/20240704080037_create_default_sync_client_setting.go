package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := db.NewSelect().
			Model(&database.SyncClientSetting{}).
			Scan(ctx)
		if err == nil {
			return nil
		}

		syncClientSetting := database.SyncClientSetting{
			Token:           "225caecb203219c972ae8d4368b93f868e6aed5a",
			ConcurrentCount: 1,
			MaxBandwidth:    8192000,
			IsDefault:       true,
		}
		return db.NewInsert().
			Model(&syncClientSetting).
			Scan(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}
