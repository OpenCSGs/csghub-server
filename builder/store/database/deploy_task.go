package database

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/types"
)

type Deploy struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// space_id to deploy, it's 0 if deploy model
	SpaceID   int64  `bun:",notnull" json:"space_id"`
	Status    int    `bun:",notnull" json:"status"`
	GitPath   string `bun:",notnull" json:"git_path"`
	GitBranch string `bun:",notnull" json:"git_branch"`
	Env       string `bun:",nullzero" json:"env"`
	Secret    string `bun:",nullzero" json:"secret"`
	Template  string `bun:",notnull" json:"tmeplate"`
	Hardware  string `bun:",notnull" json:"hardware"`
	// for image run task, aka task_type = 1
	// running image of cluster, comes from builder or pre-define
	ImageID string `bun:",nullzero" json:"image_id"`
	// add at 2024-05
	DeployName string `json:"deploy_name"`
	// user_id trigger deploy action, rather than repo owner user_id
	UserID int64 `json:"user_id"`
	// model_id to deploy, it's 0 if deploy space
	ModelID int64 `json:"model_id"`
	// repository_id of model/space/code/dataset
	RepoID int64 `json:"repo_id"`
	// model running engine vllm or TGI
	RuntimeFramework string `bun:",nullzero" json:"runtime_framework"`
	Annotation       string `bun:",nullzero" json:"annotation"`
	MinReplica       int    `json:"min_replica"`
	MaxReplica       int    `json:"max_replica"`
	SvcName          string `json:"svc_name"`
	Endpoint         string `json:"endpoint"`
	// minimum unit of money for counting cost
	CostPerHour int64  `json:"cost_per_hour"`
	ClusterID   string `json:"cluster_id"`
	SecureLevel int    `json:"secure_level"`
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

func (s *DeployTaskStore) GetLatestDeployBySpaceID(ctx context.Context, spaceID int64) (*Deploy, error) {
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

func (s *DeployTaskStore) ListDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64) ([]Deploy, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("repo_id = ? and user_id = ?", repoID, userID)
	if repoType == types.ModelRepo {
		query = query.Where("status != ?", common.Deleted)
	}
	query = query.Order("id desc")
	if repoType == types.SpaceRepo {
		query = query.Limit(1)
	}
	_, err := query.Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DeployTaskStore) DeleteDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64, deployID int64) error {
	// only delete the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Update deploys set status = ? where id = ? and repo_id = ? and user_id = ?", common.Deleted, deployID, repoID, userID)
	if err != nil {
		return err
	}
	err = assertAffectedOneRow(res, err)
	return err
}
