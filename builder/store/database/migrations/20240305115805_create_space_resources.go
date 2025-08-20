package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type SpaceResource struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	Name      string `bun:",notnull" json:"name"`
	Resources string `bun:",notnull" json:"resources"`
	ClusterID string `bun:",notnull" json:"cluster_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, SpaceResource{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SpaceResource{})
	})
}
