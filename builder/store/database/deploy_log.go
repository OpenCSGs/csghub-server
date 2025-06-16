package database

import (
	"context"
	"fmt"
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

type deployLogStoreImpl struct {
	db *DB
}

func NewDeployLogStore() DeployLogStore {
	return &deployLogStoreImpl{db: defaultDB}
}

func NewDeployTaskLogWithDB(db *DB) DeployLogStore {
	return &deployLogStoreImpl{db: db}
}

type DeployLogStore interface {
	UpdateDeployLogs(ctx context.Context, log DeployLog) (*DeployLog, error)
	GetDeployLogs(ctx context.Context, log DeployLog) (*DeployLog, error)
}

func (s *deployLogStoreImpl) UpdateDeployLogs(ctx context.Context, log DeployLog) (*DeployLog, error) {
	_, err := s.db.Core.NewInsert().Model(&log).On("CONFLICT (cluster_id, svc_name, pod_name) DO UPDATE").
		Set("pod_status = ?", log.PodStatus).
		Set("user_container_log = ?", log.UserContainerLog).
		Set("deploy_id = ?", log.DeployID).
		Set("updated_at = now()").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update svc %s pod %s logs, error: %w", log.SvcName, log.PodName, err)
	}
	return &log, nil
}

func (s *deployLogStoreImpl) GetDeployLogs(ctx context.Context, log DeployLog) (*DeployLog, error) {
	var result DeployLog
	q := s.db.Core.NewSelect().Model(&result).
		Where("cluster_id = ?", log.ClusterID).
		Where("svc_name = ?", log.SvcName)

	if len(log.PodName) > 0 {
		q.Where("pod_name = ?", log.PodName)
	}

	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get svc %s pod %s logs, error: %w", log.SvcName, log.PodName, err)
	}
	return &result, nil
}
