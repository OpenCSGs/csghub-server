package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type ClusterInfoStore struct {
	db *DB
}

func NewClusterInfoStore() *ClusterInfoStore {
	return &ClusterInfoStore{
		db: defaultDB,
	}
}

type ClusterInfo struct {
	ClusterID     string `bun:",pk" json:"cluster_id"`
	ClusterConfig string `bun:",notnull" json:"cluster_config"`
	Region        string `bun:",notnull" json:"region"`
}

func (r *ClusterInfoStore) Add(ctx context.Context, clusterConfig string, region string) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		cluster := &ClusterInfo{
			ClusterID:     uuid.New().String(),
			ClusterConfig: clusterConfig,
			Region:        region,
		}

		_, err := r.ByClusterConfig(ctx, clusterConfig)
		if err != nil {
			if err := assertAffectedOneRow(tx.NewInsert().Model(cluster).Exec(ctx)); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (r *ClusterInfoStore) Update(ctx context.Context, clusterConfig string, region string) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		cluster, err := r.ByClusterConfig(ctx, clusterConfig)
		if err == nil {
			cluster.Region = region
			_, err = tx.NewUpdate().Model(cluster).
				WherePK().Exec(ctx)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update deploy,%w", err)
			}

		}
		return nil
	})
	return err
}

func (s *ClusterInfoStore) ByClusterID(ctx context.Context, clusterId string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterID = clusterId
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_id = ?", clusterId).Scan(ctx)
	return
}

func (s *ClusterInfoStore) ByClusterConfig(ctx context.Context, clusterConfig string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterConfig = clusterConfig
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_config = ?", clusterConfig).Scan(ctx)
	return
}
