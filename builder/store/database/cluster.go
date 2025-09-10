package database

import (
	"context"
	"database/sql"
	"errors"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type clusterInfoStoreImpl struct {
	db *DB
}

type ClusterInfoStore interface {
	Add(ctx context.Context, clusterConfig string, region string) (*ClusterInfo, error)
	AddByClusterID(ctx context.Context, clusterId string, region string) (*ClusterInfo, error)
	Update(ctx context.Context, clusterInfo ClusterInfo) error
	UpdateByClusterID(ctx context.Context, cluster types.ClusterEvent) error
	ByClusterID(ctx context.Context, clusterId string) (clusterInfo ClusterInfo, err error)
	ByClusterConfig(ctx context.Context, clusterConfig string) (clusterInfo ClusterInfo, err error)
	List(ctx context.Context) ([]ClusterInfo, error)
	BatchUpdateStatus(ctx context.Context, statusEvent *types.HearBeatEvent) error
}

func NewClusterInfoStore() ClusterInfoStore {
	return &clusterInfoStoreImpl{
		db: defaultDB,
	}
}

func NewClusterInfoStoreWithDB(db *DB) ClusterInfoStore {
	return &clusterInfoStoreImpl{
		db: db,
	}
}

type ClusterInfo struct {
	ClusterID        string              `bun:",pk" json:"cluster_id"`
	ClusterConfig    string              `bun:",notnull" json:"cluster_config"`
	StorageClass     string              `bun:"," json:"storage_class"`
	Region           string              `bun:"," json:"region"`
	Zone             string              `bun:"," json:"zone"`     //cn-beijing
	Provider         string              `bun:"," json:"provider"` //ali
	Enable           bool                `bun:",notnull" json:"enable"`
	Status           types.ClusterStatus `bun:"," json:"status"`            //running, unavailable
	Endpoint         string              `bun:"," json:"endpoint"`          //runner in k8s api endpoint
	NetworkInterface string              `bun:"," json:"network_interface"` //used for multi-host, e.g., eth0
	Mode             types.ClusterMode   `bun:"," json:"mode"`              //used for multi-host, e.g., host, bridge
	times
}

func (r *clusterInfoStoreImpl) Add(ctx context.Context, clusterConfig string, region string) (*ClusterInfo, error) {
	cluster, err := r.ByClusterConfig(ctx, clusterConfig)
	if errors.Is(err, sql.ErrNoRows) {
		cluster = ClusterInfo{
			ClusterID:     uuid.New().String(),
			ClusterConfig: clusterConfig,
			Region:        region,
			Enable:        true,
		}
		_, err = r.db.Operator.Core.NewInsert().Model(&cluster).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}
	return &cluster, err
}

func (r *clusterInfoStoreImpl) AddByClusterID(ctx context.Context, clusterID string, region string) (*ClusterInfo, error) {
	cluster, err := r.ByClusterID(ctx, clusterID)
	if errors.Is(err, sql.ErrNoRows) {
		cluster = ClusterInfo{
			ClusterID:     clusterID,
			ClusterConfig: types.DefaultClusterCongfig,
			Region:        region,
			Enable:        true,
		}
		_, err = r.db.Operator.Core.NewInsert().Model(&cluster).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}
	return &cluster, err
}

func (r *clusterInfoStoreImpl) Update(ctx context.Context, clusterInfo ClusterInfo) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := r.ByClusterConfig(ctx, clusterInfo.ClusterConfig)
		if err == nil {
			return assertAffectedOneRow(r.db.Operator.Core.NewUpdate().Model(&clusterInfo).WherePK().Exec(ctx))
		}
		return nil
	})
	return err
}

func (r *clusterInfoStoreImpl) UpdateByClusterID(ctx context.Context, event types.ClusterEvent) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		clusterInfo, err := r.ByClusterID(ctx, event.ClusterID)
		if err == nil {
			clusterInfo.Region = event.Region
			clusterInfo.ClusterConfig = event.ClusterConfig
			clusterInfo.Zone = event.Zone
			clusterInfo.Provider = event.Provider
			clusterInfo.Endpoint = event.Endpoint
			clusterInfo.StorageClass = event.StorageClass
			clusterInfo.NetworkInterface = event.NetworkInterface
			clusterInfo.Mode = event.Mode
			return assertAffectedOneRow(r.db.Operator.Core.NewUpdate().Model(&clusterInfo).WherePK().Exec(ctx))
		}
		return nil
	})
	return err
}

func (s *clusterInfoStoreImpl) ByClusterID(ctx context.Context, clusterId string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterID = clusterId
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_id = ?", clusterId).Scan(ctx)
	return
}

func (s *clusterInfoStoreImpl) ByClusterConfig(ctx context.Context, clusterConfig string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterConfig = clusterConfig
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_config = ?", clusterConfig).Scan(ctx)
	return
}

func (s *clusterInfoStoreImpl) List(ctx context.Context) ([]ClusterInfo, error) {
	var result []ClusterInfo
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Order("region").Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *clusterInfoStoreImpl) BatchUpdateStatus(ctx context.Context, statusEvent *types.HearBeatEvent) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		if len(statusEvent.Running) > 0 {
			_, err := tx.NewUpdate().Model(&ClusterInfo{}).
				Set("Status = ?", types.ClusterStatusRunning).
				Set("updated_at = now()").
				Where("cluster_id IN (?)", bun.In(statusEvent.Running)).
				Exec(ctx)

			if err != nil {
				return errorx.HandleDBError(err, nil)
			}
		}

		if len(statusEvent.Unavailable) > 0 {
			_, err := tx.NewUpdate().Model(&ClusterInfo{}).
				Set("Status = ?", types.ClusterStatusUnavailable).
				Set("updated_at = now()").
				Where("cluster_id IN (?)", bun.In(statusEvent.Unavailable)).
				Exec(ctx)

			if err != nil {
				return errorx.HandleDBError(err, nil)
			}
		}

		return nil
	})

	return err
}
