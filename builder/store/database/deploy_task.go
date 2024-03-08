package database

import (
	"context"
	"fmt"
	"time"
)

type Deploy struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	Status    int    `bun:",notnull" json:"status"`
	GitPath   string `bun:",notnull" json:"git_path"`
	GitBranch string `bun:",notnull" json:"git_branch"`
	Env       string `bun:",nullzero" json:"env"`
	Secret    string `bun:",nullzero" json:"secret"`
	Template  string `bun:",nonnull" json:"tmeplate"`
	Hardware  string `bun:",nonnull" json:"hardware"`
	times
}

type DeployTask struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// 0: build, 1: run
	TaskType int     `bun:",notnull" json:"task_type"`
	Status   int     `bun:",notnull" json:"status"`
	Message  string  `bun:",nullzero" json:"message"`
	DeployID int64   `bun:",notnull" json:"deploy_id"`
	Deploy   *Deploy `bun:"rel:belongs-to,join:deploy_id=id" json:"deploy"`
	times
}

type MonitorTask struct {
	DeployTaskID int64 `bun:",pk" json:"deploy_task_id"`
	// DeployID     int64     `bun:",notnull" json:"deploy_id"`
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	*DeployTask
}

type DeployTaskStore struct {
	db *DB
}

func NewDeployTaskStore() *DeployTaskStore {
	return &DeployTaskStore{db: defaultDB}
}

func (s *DeployTaskStore) CreateDeploy(ctx context.Context, deploy *Deploy) error {
	_, err := s.db.Core.NewInsert().Model(deploy).Exec(ctx, deploy)
	return err
}

func (s *DeployTaskStore) UpdateDeploy(ctx context.Context, deploy *Deploy) error {
	_, err := s.db.Core.NewUpdate().Model(deploy).WherePK().Exec(ctx)
	return err
}

func (s *DeployTaskStore) GetDeploy(ctx context.Context, id int64) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Core.NewSelect().Model(deploy).Where("id = ?", id).Limit(1).Scan(ctx, deploy)
	return deploy, err
}

func (s *DeployTaskStore) CreateDeployTask(ctx context.Context, deployTask *DeployTask) error {
	_, err := s.db.Core.NewInsert().Model(deployTask).Exec(ctx, deployTask)
	return err
}

func (s *DeployTaskStore) UpdateDeployTask(ctx context.Context, deployTask *DeployTask) error {
	_, err := s.db.Core.NewUpdate().Model(deployTask).WherePK().Exec(ctx)
	return err
}

func (s *DeployTaskStore) GetDeployTask(ctx context.Context, id int64) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Where("id = ?", id).Relation("Deploy").Limit(1).Scan(ctx, deployTask)
	return deployTask, err
}

func (s *DeployTaskStore) GetDeployTasksOfDeploy(ctx context.Context, deployID int64) ([]*DeployTask, error) {
	var deployTasks []*DeployTask
	err := s.db.Core.NewSelect().Model((*DeployTask)(nil)).Where("deploy_id = ?", deployID).Scan(ctx, &deployTasks)
	return deployTasks, err
}

// GetNewTaskAfter return the first task of the next deploy
func (s *DeployTaskStore) GetNewTaskAfter(ctx context.Context, currentDeployID int64) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Where("deploy_id > ? and status = 0", currentDeployID).Order("id ASC").Limit(1).Scan(ctx, deployTask)
	return deployTask, err
}

// GetNewTaskFirst returns the first task which has never scheduled, or status is 0
func (s *DeployTaskStore) GetNewTaskFirst(ctx context.Context) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Where("status = ?", 0).Order("id asc").Limit(1).Scan(ctx, deployTask)
	return deployTask, err
}

func (s *DeployTaskStore) CreateMonitorTask(ctx context.Context, monitorTask *MonitorTask) error {
	_, err := s.db.Core.NewInsert().Model(monitorTask).Exec(ctx)
	return err
}

func (s *DeployTaskStore) GetAllMonitorTasks(ctx context.Context) ([]*MonitorTask, error) {
	var monitorTasks []*MonitorTask
	err := s.db.Core.NewSelect().Model(&monitorTasks).Scan(ctx)
	return monitorTasks, err
}

func (s *DeployTaskStore) DeleteMonitorTask(ctx context.Context, deployTaskID int64) error {
	_, err := s.db.Core.NewDelete().Model((*MonitorTask)(nil)).Where("deploy_task_id = ?", deployTaskID).Exec(ctx)
	return err
}

func (s *DeployTaskStore) UpdateInTx(ctx context.Context, deploy *Deploy, deployTasks ...*DeployTask) error {
	tx, err := s.db.Core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction,%w", err)
	}
	_, err = tx.NewUpdate().Model(deploy).WherePK().Exec(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update deploy,%w", err)
	}
	_, err = tx.NewUpdate().Model(&deployTasks).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update deploy task,%w", err)
	}

	return tx.Commit()
}
