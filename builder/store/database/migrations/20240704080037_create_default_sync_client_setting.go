package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type SyncClientSetting struct {
	ID              int64  `bun:",pk,autoincrement" json:"id"`
	Token           string `bun:",notnull" json:"token"`
	ConcurrentCount int    `bun:",nullzero" json:"concurrent_count"`
	MaxBandwidth    int    `bun:",nullzero" json:"max_bandwidth"`
	IsDefault       bool   `bun:"," json:"default"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := db.NewSelect().
			Model(&SyncClientSetting{}).
			Scan(ctx)
		if err == nil {
			return nil
		}

		syncClientSetting := SyncClientSetting{
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
