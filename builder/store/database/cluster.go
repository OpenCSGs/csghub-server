package database

import (
	"context"

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
		if err := assertAffectedOneRow(tx.NewInsert().Model(cluster).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Exec("update cluster_infos set region=? where cluster_config=?", region, clusterConfig)); err != nil {
			return err
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
