package migrations

import (
	"context"
	"fmt"
	"go/types"

	"github.com/uptrace/bun"
	corev1 "k8s.io/api/core/v1"
)

type KnativeService struct {
	ID             int64                  `bun:",pk,autoincrement" json:"id"`
	Name           string                 `bun:",notnull" json:"name"`
	Status         corev1.ConditionStatus `bun:",notnull" json:"status"`
	Code           int                    `bun:",notnull" json:"code"`
	ClusterID      string                 `bun:",notnull" json:"cluster_id"`
	Endpoint       string                 `bun:"," json:"endpoint"`
	ActualReplica  int                    `bun:"," json:"actual_replica"`
	DesiredReplica int                    `bun:"," json:"desired_replica"`
	Instances      []types.Instance       `bun:"type:jsonb" json:"instances,omitempty"`
	UserUUID       string                 `bun:"," json:"user_uuid"`
	DeployID       int64                  `bun:"," json:"deploy_id"`
	DeployType     int                    `bun:"," json:"deploy_type"`
	DeploySKU      string                 `bun:"," json:"deploy_sku"`
	OrderDetailID  int64                  `bun:"," json:"order_detail_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, KnativeService{})
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, "ALTER TABLE knative_services ADD CONSTRAINT unique_cluster_svc UNIQUE (cluster_id, name)")
		if err != nil {
			return fmt.Errorf("failed to add unique for knative_services table: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*KnativeService)(nil)).
			Index("idx_knative_name_cluster").
			Column("name", "cluster_id").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("fail to create index idx_knative_name_cluster_user : %w", err)
		}
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, KnativeService{})
	})
}
