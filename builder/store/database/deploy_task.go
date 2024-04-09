package database

import (
	"context"
	"fmt"
	"time"
)

type Deploy struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	SpaceID   int64  `bun:",notnull" json:"space_id"`
	Status    int    `bun:",notnull" json:"status"`
	GitPath   string `bun:",notnull" json:"git_path"`
	GitBranch string `bun:",notnull" json:"git_branch"`
	Env       string `bun:",nullzero" json:"env"`
	Secret    string `bun:",nullzero" json:"secret"`
	Template  string `bun:",notnull" json:"tmeplate"`
	Hardware  string `bun:",notnull" json:"hardware"`
	// for image run task, aka task_type = 1
	ImageID string `bun:",nullzero" json:"image_id"`
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

func (s *DeployTaskStore) GetSpaceLatestDeploy(ctx context.Context, spaceID int64) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Core.NewSelect().Model(deploy).Where("space_id = ?", spaceID).Order("created_at DESC").Limit(1).Scan(ctx, deploy)
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
	err := s.db.Core.NewSelect().Model(deployTask).Where("deploy_task.id = ?", id).
		Relation("Deploy").
		Limit(1).
		Scan(ctx, deployTask)
	return deployTask, err
}

func (s *DeployTaskStore) GetDeployTasksOfDeploy(ctx context.Context, deployID int64) ([]*DeployTask, error) {
	var deployTasks []*DeployTask
	err := s.db.Core.NewSelect().Model((*DeployTask)(nil)).Where("deploy_id = ?", deployID).Scan(ctx, &deployTasks)
	return deployTasks, err
}

// GetNewTaskAfter return the first task of the next deploy
func (s *DeployTaskStore) GetNewTaskAfter(ctx context.Context, currentDeployTaskID int64) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Relation("Deploy").
		Where("deploy_task.id > ? ", currentDeployTaskID).
		Where("(task_type = 0 and deploy_task.status in (0,1)) or (task_type = 1 and deploy_task.status in (0,1,3))").
		Order("id ASC").
		Limit(1).
		Scan(ctx, deployTask)
	return deployTask, err
}

// GetNewTaskFirst returns the first task which has  not end
func (s *DeployTaskStore) GetNewTaskFirst(ctx context.Context) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Relation("Deploy").
		Where("(task_type = 0 and deploy_task.status in (0,1)) or (task_type = 1 and deploy_task.status in (0,1,3))").
		Order("id asc").
		Limit(1).
		Scan(ctx, deployTask)
	return deployTask, err
}

func (s *DeployTaskStore) UpdateInTx(ctx context.Context, deployColumns, deployTaskColumns []string, deploy *Deploy, deployTasks ...*DeployTask) error {
	tx, err := s.db.Core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction,%w", err)
	}

	if deploy != nil {
		deployColumns = append(deployColumns, "updated_at")
		deploy.UpdatedAt = time.Now()
		_, err = tx.NewUpdate().Model(deploy).
			Column(deployColumns...).
			WherePK().Exec(ctx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update deploy,%w", err)
		}
	}

	for _, t := range deployTasks {
		t.UpdatedAt = time.Now()
	}
	deployTaskColumns = append(deployTaskColumns, "updated_at")
	_, err = tx.NewUpdate().
		Model(&deployTasks).
		// Column("status", "message", "updated_at").
		Column(deployTaskColumns...).
		Bulk().
		Exec(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update deploy tasks in tx,%w", err)
	}

	return tx.Commit()
}
