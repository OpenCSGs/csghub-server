package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type ClusterNodeOwnership struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	ClusterNodeID int64  `bun:",notnull" json:"cluster_node_id"`
	ClusterID     string `bun:",notnull" json:"cluster_id"`
	UserUUID      string `bun:",nullzero" json:"user_uuid"`
	OrgUUID       string `bun:",nullzero" json:"org_uuid"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, ClusterNodeOwnership{})
		if err != nil {
			return fmt.Errorf("create table cluster_node_ownerships fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*ClusterNodeOwnership)(nil)).
			Index("idx_cluster_node_ownership_node_id").
			Column("cluster_node_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_cluster_node_ownership_node_id fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*ClusterNodeOwnership)(nil)).
			Index("idx_cluster_node_ownership_cluster_id").
			Column("cluster_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_cluster_node_ownership_cluster_id fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*ClusterNodeOwnership)(nil)).
			Index("idx_cluster_node_ownership_user_uuid").
			Column("user_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_cluster_node_ownership_user_uuid fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*ClusterNodeOwnership)(nil)).
			Index("idx_cluster_node_ownership_org_uuid").
			Column("org_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_cluster_node_ownership_org_uuid fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ClusterNodeOwnership{})
	})
}
