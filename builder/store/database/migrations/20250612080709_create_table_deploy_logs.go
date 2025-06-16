package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type DeployLog struct {
	ID               int64  `bun:",pk,autoincrement" json:"id"`
	DeployID         int64  `bun:",notnull" json:"deploy_id"`
	ClusterID        string `bun:",notnull" json:"cluster_id"`
	SvcName          string `bun:",notnull" json:"svc_name"`
	PodName          string `bun:",notnull" json:"pod_name"`
	PodStatus        string `bun:",nullzero" json:"pod_status"`
	UserContainerLog string `bun:",nullzero" json:"user_container_log"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, DeployLog{}); err != nil {
			return fmt.Errorf("failed to create table deploy logs, error: %w", err)
		}
		_, err := db.NewCreateIndex().
			Model(&DeployLog{}).
			Index("idx_deploy_logs_clusterid_svcname_podname").
			Unique().
			Column("cluster_id", "svc_name", "pod_name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, DeployLog{})
	})
}
