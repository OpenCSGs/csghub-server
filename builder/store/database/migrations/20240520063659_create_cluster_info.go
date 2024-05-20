package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type ClusterInfo struct {
	ClusterID     string `bun:",pk" json:"cluster_id"`
	ClusterConfig string `bun:",notnull" json:"cluster_config"`
	Region        string `bun:",notnull" json:"region"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, ClusterInfo{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ClusterInfo{})
	})
}
