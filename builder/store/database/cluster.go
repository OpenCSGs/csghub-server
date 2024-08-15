package database

import (
	"context"
	"database/sql"
	"errors"

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
	StorageClass  string `bun:",notnull" json:"storage_class"`
	Region        string `bun:",notnull" json:"region"`
	Zone          string `bun:",notnull" json:"zone"`     //cn-beijing
	Provider      string `bun:",notnull" json:"provider"` //ali
	Enable        bool   `bun:",notnull" json:"enable"`
}

func (r *ClusterInfoStore) Add(ctx context.Context, clusterConfig string, region string) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		cluster := &ClusterInfo{
			ClusterID:     uuid.New().String(),
			ClusterConfig: clusterConfig,
			Region:        region,
			Enable:        true,
		}

		_, err := r.ByClusterConfig(ctx, clusterConfig)
		if errors.Is(err, sql.ErrNoRows) {
			return assertAffectedOneRow(r.db.Operator.Core.NewInsert().Model(cluster).Exec(ctx))
		}
		return err
	})
	return err
}

func (r *ClusterInfoStore) Update(ctx context.Context, clusterInfo ClusterInfo) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := r.ByClusterConfig(ctx, clusterInfo.ClusterConfig)
		if err == nil {
			return assertAffectedOneRow(r.db.Operator.Core.NewUpdate().Model(&clusterInfo).WherePK().Exec(ctx))
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

func (s *ClusterInfoStore) List(ctx context.Context) ([]ClusterInfo, error) {
	var result []ClusterInfo
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Order("region").Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
