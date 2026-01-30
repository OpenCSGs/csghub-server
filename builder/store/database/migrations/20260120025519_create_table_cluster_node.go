package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type ClusterNode struct {
	ID          int64               `bun:",pk,autoincrement" json:"id"`
	ClusterID   string              `bun:",notnull" json:"cluster_id"`
	Name        string              `bun:",notnull" json:"name"`
	Status      string              `bun:",nullzero" json:"status"`
	Labels      map[string]string   `bun:",type:jsonb,nullzero" json:"labels"`
	EnableVXPU  bool                `bun:",default:false" json:"enable_vxpu"`
	ComputeCard string              `bun:",nullzero" json:"compute_card"`
	Hardware    map[string]any      `bun:",type:jsonb,nullzero" json:"hardware"`
	Processes   []types.ProcessInfo `bun:",type:jsonb,nullzero" json:"processes"`
	Exclusive   bool                `bun:",default:false" json:"exclusive"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, ClusterNode{}); err != nil {
			return fmt.Errorf("create table cluster_node fail: %w", err)
		}
		_, err := db.NewCreateIndex().
			Model((*ClusterNode)(nil)).
			Index("idx_cluster_id_name").
			Unique().
			Column("cluster_id", "name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create unique index fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ClusterNode{})
	})
}
