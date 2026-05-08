package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type DeployBenchmarkTask struct {
	ID                 int64      `bun:",pk,autoincrement" json:"id"`
	DeployID           int64      `bun:",notnull" json:"deploy_id"`
	SourceDeployTaskID int64      `bun:",nullzero" json:"source_deploy_task_id"`
	WorkflowID         string     `bun:",nullzero" json:"workflow_id"`
	TriggerSource      string     `bun:",notnull" json:"trigger_source"`
	TriggerKey         string     `bun:",notnull" json:"trigger_key"`
	BenchmarkType      string     `bun:",notnull" json:"benchmark_type"`
	Status             string     `bun:",notnull" json:"status"`
	RuntimeFramework   string     `bun:",notnull" json:"runtime_framework"`
	Task               string     `bun:",notnull" json:"task"`
	Endpoint           string     `bun:",notnull" json:"endpoint"`
	SvcName            string     `bun:",notnull" json:"svc_name"`
	ClusterID          string     `bun:",notnull" json:"cluster_id"`
	OwnerNamespace     string     `bun:",notnull" json:"owner_namespace"`
	UserUUID           string     `bun:",notnull" json:"user_uuid"`
	Hardware           map[string]any              `bun:"type:jsonb,notnull" json:"hardware"`
	RequestTemplate    types.DeployBenchmarkTemplate `bun:"type:jsonb,notnull" json:"request_template"`
	BenchmarkConfig    types.DeployBenchmarkConfig   `bun:"type:jsonb,notnull" json:"benchmark_config"`
	ResultSummary      types.DeployBenchmarkSummary  `bun:"type:jsonb,notnull" json:"result_summary"`
	RawResult          map[string]any               `bun:"type:jsonb,notnull" json:"raw_result"`
	ErrorMessage       string     `bun:",type:text,nullzero" json:"error_message"`
	StartedAt          *time.Time `bun:",nullzero" json:"started_at,omitempty"`
	FinishedAt         *time.Time `bun:",nullzero" json:"finished_at,omitempty"`
	times
}

type DeployBenchmarkTaskStore interface {
	Create(ctx context.Context, task *DeployBenchmarkTask) (*DeployBenchmarkTask, error)
	Update(ctx context.Context, task *DeployBenchmarkTask) error
	FindByID(ctx context.Context, id int64) (*DeployBenchmarkTask, error)
	FindLatestByDeployID(ctx context.Context, deployID int64) (*DeployBenchmarkTask, error)
	FindByTrigger(ctx context.Context, deployID int64, triggerSource, triggerKey string) (*DeployBenchmarkTask, error)
	ListByDeployID(ctx context.Context, deployID int64, per, page int) ([]DeployBenchmarkTask, int, error)
}

type deployBenchmarkTaskStoreImpl struct {
	db *DB
}

func NewDeployBenchmarkTaskStore() DeployBenchmarkTaskStore {
	return &deployBenchmarkTaskStoreImpl{db: defaultDB}
}

func NewDeployBenchmarkTaskStoreWithDB(db *DB) DeployBenchmarkTaskStore {
	return &deployBenchmarkTaskStoreImpl{db: db}
}

func (s *deployBenchmarkTaskStoreImpl) Create(ctx context.Context, task *DeployBenchmarkTask) (*DeployBenchmarkTask, error) {
	res, err := s.db.Core.NewInsert().Model(task).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"deploy_id": task.DeployID,
		})
	}
	return task, nil
}

func (s *deployBenchmarkTaskStoreImpl) Update(ctx context.Context, task *DeployBenchmarkTask) error {
	res, err := s.db.Core.NewUpdate().Model(task).Where("id = ?", task.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"benchmark_task_id": task.ID,
		})
	}
	return nil
}

func (s *deployBenchmarkTaskStoreImpl) FindByID(ctx context.Context, id int64) (*DeployBenchmarkTask, error) {
	task := &DeployBenchmarkTask{}
	err := s.db.Core.NewSelect().Model(task).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"benchmark_task_id": id,
		})
	}
	return task, nil
}

func (s *deployBenchmarkTaskStoreImpl) FindLatestByDeployID(ctx context.Context, deployID int64) (*DeployBenchmarkTask, error) {
	task := &DeployBenchmarkTask{}
	err := s.db.Core.NewSelect().
		Model(task).
		Where("deploy_id = ?", deployID).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"deploy_id": deployID,
		})
	}
	return task, nil
}

func (s *deployBenchmarkTaskStoreImpl) FindByTrigger(ctx context.Context, deployID int64, triggerSource, triggerKey string) (*DeployBenchmarkTask, error) {
	task := &DeployBenchmarkTask{}
	err := s.db.Core.NewSelect().
		Model(task).
		Where("deploy_id = ?", deployID).
		Where("trigger_source = ?", triggerSource).
		Where("trigger_key = ?", triggerKey).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"deploy_id":      deployID,
			"trigger_source": triggerSource,
			"trigger_key":    triggerKey,
		})
	}
	return task, nil
}

func (s *deployBenchmarkTaskStoreImpl) ListByDeployID(ctx context.Context, deployID int64, per, page int) ([]DeployBenchmarkTask, int, error) {
	var tasks []DeployBenchmarkTask
	query := s.db.Core.NewSelect().
		Model(&tasks).
		Where("deploy_id = ?", deployID)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{"deploy_id": deployID})
	}

	if per > 0 && page > 0 {
		query = query.Limit(per).Offset((page - 1) * per)
	}

	err = query.Order("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{"deploy_id": deployID})
	}
	return tasks, total, nil
}

func DeployBenchmarkTaskToResp(task *DeployBenchmarkTask) (*types.DeployBenchmarkResp, error) {
	if task == nil {
		return nil, fmt.Errorf("deploy benchmark task is nil")
	}

	return &types.DeployBenchmarkResp{
		ID:                 task.ID,
		DeployID:           task.DeployID,
		SourceDeployTaskID: task.SourceDeployTaskID,
		WorkflowID:         task.WorkflowID,
		TriggerSource:      task.TriggerSource,
		Status:             task.Status,
		BenchmarkType:      task.BenchmarkType,
		RuntimeFramework:   task.RuntimeFramework,
		Task:               task.Task,
		Endpoint:           task.Endpoint,
		Summary:            task.ResultSummary,
		BenchmarkConfig:    task.BenchmarkConfig,
		RequestTemplate:    task.RequestTemplate,
		ErrorMessage:       task.ErrorMessage,
		StartedAt:          task.StartedAt,
		FinishedAt:         task.FinishedAt,
		CreatedAt:          task.CreatedAt,
		UpdatedAt:          task.UpdatedAt,
	}, nil
}

// ShouldSkipBenchmark checks if benchmark should be skipped for the given deploy
// Returns (shouldSkip, reason) where reason explains why it was skipped
func ShouldSkipBenchmark(deployReq types.DeployRequest) (bool, string) {
	if strings.TrimSpace(deployReq.Endpoint) == "" {
		return true, "endpoint is empty"
	}

	if !types.IsDeployBenchmarkTaskSupported(types.PipelineTask(deployReq.Task)) {
		return true, fmt.Sprintf("unsupported task type: %s", deployReq.Task)
	}

	return false, ""
}

// CreateSkippedBenchmarkTask creates a benchmark task with skipped status
func CreateSkippedBenchmarkTask(ctx context.Context, store DeployBenchmarkTaskStore, req types.DeployBenchmarkLaunchReq, reason string) (*DeployBenchmarkTask, error) {
	now := time.Now()
	benchmarkType, ok := types.ResolveDeployBenchmarkType(types.PipelineTask(req.Deploy.Task))
	if !ok {
		benchmarkType = ""
	}
	task := &DeployBenchmarkTask{
		DeployID:           req.Deploy.DeployID,
		SourceDeployTaskID: req.SourceDeployTaskID,
		TriggerSource:      req.TriggerSource,
		TriggerKey:         req.TriggerKey,
		BenchmarkType:      benchmarkType,
		Status:             types.DeployBenchmarkStatusSkipped,
		RuntimeFramework:   req.Deploy.RuntimeFramework,
		Task:               req.Deploy.Task,
		Endpoint:           req.Deploy.Endpoint,
		SvcName:            req.Deploy.SvcName,
		ClusterID:          req.Deploy.ClusterID,
		OwnerNamespace:     req.Deploy.OwnerNamespace,
		UserUUID:           req.Deploy.UserUUID,
		Hardware:           map[string]any{},
		RequestTemplate:    types.DeployBenchmarkTemplate{},
		BenchmarkConfig:    types.DeployBenchmarkConfig{},
		ResultSummary:      types.DeployBenchmarkSummary{},
		RawResult:          map[string]any{},
		ErrorMessage:       reason,
		StartedAt:          &now,
		FinishedAt:         &now,
	}

	created, err := store.Create(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("create skipped benchmark task: %w", err)
	}
	return created, nil
}
