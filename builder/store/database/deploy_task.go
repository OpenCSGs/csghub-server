package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/errorx"
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
	Template  string `bun:",notnull" json:"template"`
	Hardware  string `bun:",notnull" json:"hardware"`
	// for image run task, aka task_type = 1
	// running image of cluster, comes from builder or pre-define
	ImageID string `bun:",nullzero" json:"image_id"`
	// add at 2024-05
	DeployName string `json:"deploy_name"`
	// user_id trigger deploy action, rather than repo owner user_id
	UserID int64 `bun:",notnull" json:"user_id"`
	User   *User `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty"`
	// model_id to deploy, it's 0 if deploy space
	ModelID int64 `json:"model_id"`
	// repository_id of model/space/code/dataset
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `bun:"rel:belongs-to,join:repo_id=id" json:"repository,omitempty"`
	// model running engine vllm or TGI
	RuntimeFramework string `bun:",nullzero" json:"runtime_framework"`
	ContainerPort    int    `json:"container_port"`
	Annotation       string `bun:",nullzero" json:"annotation"`
	MinReplica       int    `json:"min_replica"`
	MaxReplica       int    `json:"max_replica"`
	SvcName          string `json:"svc_name"`
	Endpoint         string `json:"endpoint"`
	ClusterID        string `json:"cluster_id"`
	// 1-public, 2-private, 3-extension in future
	SecureLevel int `json:"secure_level"`
	// 0-space, 1-inference, 2-finetune, 3-serverless, 4-evaluation, 5-notebook
	Type          int                `json:"type"`
	Task          types.PipelineTask `bun:",nullzero" json:"task"` //text-generation,text-to-image
	UserUUID      string             `bun:"," json:"user_uuid"`
	SKU           string             `bun:"," json:"sku"`
	OrderDetailID int64              `bun:"," json:"order_detail_id"`
	EngineArgs    string             `bun:"," json:"engine_args"`
	Variables     string             `bun:",nullzero" json:"variables"`
	Message       string             `bun:",nullzero" json:"message"`
	Reason        string             `bun:",nullzero" json:"reason"`
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

type deployTaskStoreImpl struct {
	db *DB
}

type DeployTaskStore interface {
	CreateDeploy(ctx context.Context, deploy *Deploy) error
	UpdateDeploy(ctx context.Context, deploy *Deploy) error
	GetLatestDeployBySpaceID(ctx context.Context, spaceID int64) (*Deploy, error)
	CreateDeployTask(ctx context.Context, deployTask *DeployTask) error
	UpdateDeployTask(ctx context.Context, deployTask *DeployTask) error
	GetDeployTask(ctx context.Context, id int64) (*DeployTask, error)
	GetDeployTasksOfDeploy(ctx context.Context, deployID int64) ([]*DeployTask, error)
	// GetNewTaskAfter return the first task of the next deploy
	GetNewTaskAfter(ctx context.Context, currentDeployTaskID int64) (*DeployTask, error)
	// GetNewTaskFirst returns the first task which has  not end
	GetNewTaskFirst(ctx context.Context) (*DeployTask, error)
	UpdateInTx(ctx context.Context, deployColumns, deployTaskColumns []string, deploy *Deploy, deployTasks ...*DeployTask) error
	ListDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64) ([]Deploy, error)
	ListDeployByType(ctx context.Context, req types.DeployReq) ([]Deploy, int, error)
	DeleteDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64, deployID int64) error
	DeleteDeployNow(ctx context.Context, deployID int64) error
	DeleteDeployByID(ctx context.Context, userID int64, deployID int64) error
	ListDeployByUserID(ctx context.Context, userID int64, req *types.DeployReq) ([]Deploy, int, error)
	ListInstancesByUserID(ctx context.Context, userID int64, per, page int) ([]Deploy, int, error)
	GetDeployByID(ctx context.Context, deployID int64) (*Deploy, error)
	GetDeployBySvcName(ctx context.Context, svcName string) (*Deploy, error)
	StopDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64, deployID int64) error
	StopDeployByID(ctx context.Context, userID int64, deployID int64) error
	GetServerlessDeployByRepID(ctx context.Context, repoID int64) (*Deploy, error)
	ListServerless(ctx context.Context, req types.DeployReq) ([]Deploy, int, error)
	GetRunningDeployByUserID(ctx context.Context, userID int64) ([]Deploy, error)
	ListAllDeployByUID(ctx context.Context, userID int64) ([]Deploy, error)
	ListAllDeploys(ctx context.Context, req types.DeployReq, isActive bool) ([]Deploy, int, error)
	RunningVisibleToUser(ctx context.Context, userID int64) ([]Deploy, error)
	ListAllRunningDeploys(ctx context.Context) ([]Deploy, error)
}

func NewDeployTaskStore() DeployTaskStore {
	return &deployTaskStoreImpl{db: defaultDB}
}

func NewDeployTaskStoreWithDB(db *DB) DeployTaskStore {
	return &deployTaskStoreImpl{db: db}
}

func (s *deployTaskStoreImpl) CreateDeploy(ctx context.Context, deploy *Deploy) error {
	_, err := s.db.Core.NewInsert().Model(deploy).Exec(ctx, deploy)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) UpdateDeploy(ctx context.Context, deploy *Deploy) error {
	_, err := s.db.Core.NewUpdate().Model(deploy).WherePK().Exec(ctx)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) GetLatestDeployBySpaceID(ctx context.Context, spaceID int64) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Core.NewSelect().Model(deploy).Where("space_id = ?", spaceID).Order("created_at DESC").Limit(1).Scan(ctx, deploy)
	err = errorx.HandleDBError(err, nil)
	return deploy, err
}

func (s *deployTaskStoreImpl) CreateDeployTask(ctx context.Context, deployTask *DeployTask) error {
	_, err := s.db.Core.NewInsert().Model(deployTask).Exec(ctx, deployTask)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) UpdateDeployTask(ctx context.Context, deployTask *DeployTask) error {
	_, err := s.db.Core.NewUpdate().Model(deployTask).WherePK().Exec(ctx)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) GetDeployTask(ctx context.Context, id int64) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Where("deploy_task.id = ?", id).
		Relation("Deploy").
		Limit(1).
		Scan(ctx, deployTask)
	err = errorx.HandleDBError(err, nil)
	return deployTask, err
}

func (s *deployTaskStoreImpl) GetDeployTasksOfDeploy(ctx context.Context, deployID int64) ([]*DeployTask, error) {
	var deployTasks []*DeployTask
	err := s.db.Core.NewSelect().Model((*DeployTask)(nil)).Where("deploy_id = ?", deployID).Scan(ctx, &deployTasks)
	err = errorx.HandleDBError(err, nil)
	return deployTasks, err
}

// GetNewTaskAfter return the first task of the next deploy
func (s *deployTaskStoreImpl) GetNewTaskAfter(ctx context.Context, currentDeployTaskID int64) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Relation("Deploy").
		Where("deploy_task.id > ? ", currentDeployTaskID).
		Where("(task_type = 0 and deploy_task.status in (0,1)) or (task_type = 1 and deploy_task.status in (0,1,3))").
		Order("id ASC").
		Limit(1).
		Scan(ctx, deployTask)
	err = errorx.HandleDBError(err, nil)
	return deployTask, err
}

// GetNewTaskFirst returns the first task which has  not end
func (s *deployTaskStoreImpl) GetNewTaskFirst(ctx context.Context) (*DeployTask, error) {
	deployTask := &DeployTask{}
	err := s.db.Core.NewSelect().Model(deployTask).Relation("Deploy").
		Where("(task_type = 0 and deploy_task.status in (0,1)) or (task_type = 1 and deploy_task.status in (0,1,3))").
		Order("id asc").
		Limit(1).
		Scan(ctx, deployTask)
	err = errorx.HandleDBError(err, nil)
	return deployTask, err
}

func (s *deployTaskStoreImpl) UpdateInTx(ctx context.Context, deployColumns, deployTaskColumns []string, deploy *Deploy, deployTasks ...*DeployTask) error {
	tx, err := s.db.Core.BeginTx(ctx, nil)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction,%w", err)
	}

	if deploy != nil {
		deployColumns = append(deployColumns, "updated_at")
		deploy.UpdatedAt = time.Now()
		_, err = tx.NewUpdate().Model(deploy).
			Column(deployColumns...).
			WherePK().Exec(ctx)
		err = errorx.HandleDBError(err, nil)
		if err != nil {
			if ree := tx.Rollback(); ree != nil {
				slog.Error("rollback failed", "error", err)
			}
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
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		if ree := tx.Rollback(); ree != nil {
			slog.Error("rollback failed", "error", err)
		}
		return fmt.Errorf("failed to update deploy tasks in tx,%w", err)
	}

	return tx.Commit()
}

func (s *deployTaskStoreImpl) ListDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64) ([]Deploy, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("user_id = ? and repo_id = ?", userID, repoID)
	if repoType == types.ModelRepo {
		query = query.Where("status != ?", common.Deleted)
	}
	query = query.Order("id desc")
	if repoType == types.SpaceRepo {
		query = query.Limit(1)
	}
	_, err := query.Exec(ctx, &result)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *deployTaskStoreImpl) DeleteDeployNow(ctx context.Context, deployID int64) error {
	// only delete the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Delete from deploys where id = ?", deployID)
	err = assertAffectedOneRow(res, err)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) DeleteDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64, deployID int64) error {
	// only delete the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Update deploys set status = ? where id = ? and repo_id = ? and user_id = ?", common.Deleted, deployID, repoID, userID)
	err = assertAffectedOneRow(res, err)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) DeleteDeployByID(ctx context.Context, userID int64, deployID int64) error {
	// only delete the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Update deploys set status = ? where id = ? and user_id = ?", common.Deleted, deployID, userID)
	err = assertAffectedOneRow(res, err)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) ListDeployByUserID(ctx context.Context, userID int64, req *types.DeployReq) ([]Deploy, int, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("user_id = ? and type = ?", userID, req.DeployType)
	query = query.Where("status != ?", common.Deleted)
	if req.RepoType == types.ModelRepo {
		query = query.Where("model_id > 0")
	}
	query = query.Order("id desc")
	if req.RepoType == types.SpaceRepo {
		query = query.Where("space_id > 0")
		query = query.Limit(1)
	}
	query = query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	return result, total, nil
}

func (s *deployTaskStoreImpl) ListInstancesByUserID(ctx context.Context, userID int64, per, page int) ([]Deploy, int, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("user_id = ?", userID)
	query = query.Where("type = ? and status != ?", types.FinetuneType, common.Deleted)
	query = query.Order("id desc")
	query = query.Limit(per).Offset((page - 1) * per)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	return result, total, nil
}

func (s *deployTaskStoreImpl) GetDeployByID(ctx context.Context, deployID int64) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Operator.Core.NewSelect().Model(deploy).Where("id = ?", deployID).Scan(ctx, deploy)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("deploy_id", deployID))
	return deploy, err
}

func (s *deployTaskStoreImpl) GetDeployBySvcName(ctx context.Context, svcName string) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Operator.Core.NewSelect().Model(deploy).Where("svc_name = ?", svcName).Scan(ctx, deploy)
	err = errorx.HandleDBError(err, nil)
	return deploy, err
}

func (s *deployTaskStoreImpl) StopDeploy(ctx context.Context, repoType types.RepositoryType, repoID, userID int64, deployID int64) error {
	// only stop the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Update deploys set status=?,updated_at=current_timestamp where id = ? and repo_id = ? and user_id = ?", common.Stopped, deployID, repoID, userID)
	err = assertAffectedOneRow(res, err)
	err = errorx.HandleDBError(err, nil)
	return err
}
func (s *deployTaskStoreImpl) StopDeployByID(ctx context.Context, userID int64, deployID int64) error {
	// only stop the deploy of specific repo was triggered by current login user
	res, err := s.db.BunDB.Exec("Update deploys set status=?,updated_at=current_timestamp where id = ? and user_id = ?", common.Stopped, deployID, userID)
	err = assertAffectedOneRow(res, err)
	err = errorx.HandleDBError(err, nil)
	return err
}

func (s *deployTaskStoreImpl) GetServerlessDeployByRepID(ctx context.Context, repoID int64) (*Deploy, error) {
	deploy := &Deploy{}
	err := s.db.Operator.Core.NewSelect().Model(deploy).Where("repo_id = ? and type = ?", repoID, types.ServerlessType).Scan(ctx, deploy)
	err = errorx.HandleDBError(err, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select serverless deploy error, %w", err)
	}
	return deploy, nil
}

func (s *deployTaskStoreImpl) ListServerless(ctx context.Context, req types.DeployReq) ([]Deploy, int, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("type = ?", req.DeployType)
	query = query.Where("status != ?", common.Deleted)
	query = query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	return result, total, nil
}

func (s *deployTaskStoreImpl) ListDeployByType(ctx context.Context, req types.DeployReq) ([]Deploy, int, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result).Relation("User")
	// remove invalid records
	query = query.Where("user_id != 0")
	if len(req.DeployTypes) > 0 {
		query = query.Where("type in (?)", bun.In(req.DeployTypes))
	}
	if len(req.Status) == 0 {
		query = query.Where("status != ?", common.Deleted)
	} else {
		query = query.Where("status in (?)", bun.In(req.Status))
	}

	if req.Query != "" {
		query = query.Where("deploy_name LIKE ? OR  \"user\".\"username\" LIKE ? OR cluster_id LIKE ?", "%"+req.Query+"%", "%"+req.Query+"%", "%"+req.Query+"%")
	}

	query = query.Order("created_at DESC").Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	return result, total, nil
}

type DeployFilter struct {
	UserID *int64
	Type   *int
	Status []int
}

func (s *deployTaskStoreImpl) GetDeploys(ctx context.Context, filter DeployFilter) ([]Deploy, error) {
	var result []Deploy
	q := s.db.Operator.Core.NewSelect().Model(&result)
	if filter.UserID != nil {
		q = q.Where("user_id = ?", *filter.UserID)
	}
	if filter.Type != nil {
		q = q.Where("type = ?", *filter.Type)
	}
	if len(filter.Status) > 0 {
		q = q.Where("status in (?)", bun.In(filter.Status))
	}
	_, err := q.Exec(ctx, &result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		err = errorx.HandleDBError(err, nil)
		return nil, fmt.Errorf("select deploy with filter from db failed, error:%w", err)
	}

	return result, nil
}

func (s *deployTaskStoreImpl) GetRunningDeployByUserID(ctx context.Context, userID int64) ([]Deploy, error) {
	// get all running inference and finetune of user
	var result []Deploy
	_, err := s.db.Operator.Core.NewSelect().Model(&result).
		Where("user_id = ?", userID).
		Where("type in (?)", bun.In([]int{types.SpaceType, types.InferenceType, types.FinetuneType, types.EvaluationType})).
		Where("status not in (?)", bun.In([]int{common.Stopped, common.Deleted})).
		Exec(ctx, &result)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (s *deployTaskStoreImpl) ListAllDeployByUID(ctx context.Context, userID int64) ([]Deploy, error) {
	var result []Deploy
	err := s.db.Operator.Core.NewSelect().
		Model(&result).
		Where("user_id = ? and status IN (?)", userID, bun.In([]int64{common.Running, common.Deploying, common.Building, common.BuildInQueue})).
		Scan(ctx)

	err = errorx.HandleDBError(err, nil)
	return result, err
}

func (s *deployTaskStoreImpl) RunningVisibleToUser(ctx context.Context, userID int64) ([]Deploy, error) {
	var result []Deploy
	err := s.db.Operator.Core.NewSelect().
		Model(&result).
		Relation("Repository").
		Relation("User").
		// running dedicated and serverless model inference
		Where("status = ? and type in (?)", common.Running, bun.In([]int64{types.InferenceType, types.ServerlessType})).
		WhereGroup("AND", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.WhereOr("deploy.user_id =?", userID). //user owned
									WhereOr("secure_level =?", types.EndpointPublic) // other users owned but public
		}).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return []Deploy{}, nil
	}
	err = errorx.HandleDBError(err, nil)
	return result, err
}

// list all deploy
func (s *deployTaskStoreImpl) ListAllDeploys(ctx context.Context, req types.DeployReq, isActive bool) ([]Deploy, int, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result)
	if isActive {
		query = query.Where("status != ?", common.Deleted)
	}
	query = query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, err
	}
	return result, total, nil
}

func (s *deployTaskStoreImpl) ListAllRunningDeploys(ctx context.Context) ([]Deploy, error) {
	var result []Deploy
	query := s.db.Operator.Core.NewSelect().Model(&result)
	query = query.Where("status = ?", common.Running)
	_, err := query.Exec(ctx, &result)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return result, nil
}
