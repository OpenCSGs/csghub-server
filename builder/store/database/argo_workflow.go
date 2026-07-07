package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type argoWorkFlowStoreImpl struct {
	db *DB
}

type ArgoWorkFlowStore interface {
	FindByID(ctx context.Context, id int64) (WorkFlow ArgoWorkflow, err error)
	FindByTaskID(ctx context.Context, id string) (*ArgoWorkflow, error)
	FindByUsername(ctx context.Context, username string, taskType types.TaskType, per, page int) (WorkFlows []ArgoWorkflow, total int, err error)
	FindByUsernameWithTaskTypes(ctx context.Context, username string, taskTypes []types.TaskType, per, page int) (WorkFlows []ArgoWorkflow, total int, err error)
	CreateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error)
	UpdateWorkFlowByTaskID(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error)
	// mainly for update status
	UpdateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error)
	// delete workflow by id
	DeleteWorkFlow(ctx context.Context, id int64) error
	ListAllRunningEvaluations(ctx context.Context) (WorkFlows []ArgoWorkflow, err error)
	ListRunningWorkflowsByUserUUID(ctx context.Context, userUUID string) (WorkFlows []ArgoWorkflow, err error)
	GetClusterWorkflows(ctx context.Context, req types.ClusterWFReq) ([]ArgoWorkflow, int, error)
	ListWorkflowsByTimeRange(ctx context.Context, req types.WorkflowTimeRangeReq) ([]ArgoWorkflow, int, error)
	ListWorkflowsNeedingReconcile(ctx context.Context, statuses []v1alpha1.WorkflowPhase, timeoutMin int, limit int) ([]ArgoWorkflow, error)
}

func NewArgoWorkFlowStore() ArgoWorkFlowStore {
	return &argoWorkFlowStoreImpl{
		db: defaultDB,
	}
}

func NewArgoWorkFlowStoreWithDB(db *DB) ArgoWorkFlowStore {
	return &argoWorkFlowStoreImpl{
		db: db,
	}
}

type ArgoWorkflow struct {
	ID           int64                  `bun:",pk,autoincrement" json:"id"`
	Username     string                 `bun:",notnull" json:"username"`
	UserUUID     string                 `bun:",notnull" json:"user_uuid"`
	TaskName     string                 `bun:",notnull" json:"task_name"` // user input name
	TaskId       string                 `bun:",notnull" json:"task_id"`   // generated task id
	TaskType     types.TaskType         `bun:",notnull" json:"task_type"`
	ClusterID    string                 `bun:",notnull" json:"cluster_id"`
	Namespace    string                 `bun:",notnull" json:"namespace"`
	RepoIds      []string               `bun:",notnull,type:jsonb" json:"repo_ids"`
	RepoType     string                 `bun:",notnull" json:"repo_type"`
	TaskDesc     string                 `bun:"," json:"task_desc"`
	Status       v1alpha1.WorkflowPhase `bun:"," json:"status"`
	Reason       string                 `bun:"," json:"reason"`       // reason for status
	Image        string                 `bun:",notnull" json:"image"` // ArgoWorkFlow framework
	Datasets     []string               `bun:",notnull,type:jsonb" json:"datasets"`
	ResourceId   int64                  `bun:",nullzero" json:"resource_id"`
	ResourceName string                 `bun:"," json:"resource_name"`
	StatusUpdateAt time.Time              `bun:",nullzero,notnull,default:current_timestamp" json:"status_update_at"`
	SubmitTime      time.Time              `bun:",nullzero,notnull,default:current_timestamp" json:"submit_time"`
	StartTime    time.Time              `bun:",nullzero" json:"start_time"`
	EndTime      time.Time              `bun:",nullzero" json:"end_time"`
	ResultURL    string                 `bun:"," json:"result_url"`
	DownloadURL  string                 `bun:"," json:"download_url"`
	FailuresURL  string                 `bun:"," json:"failures_url"`
	ClusterNode  string                 `bun:"," json:"cluster_node"`
	QueueName    string                 `bun:"," json:"queue_name"`
	DagTasks     string                 `bun:"," json:"dag_tasks"`
	DeletedAt    time.Time              `bun:",soft_delete,nullzero" json:"deleted_at"`
}

func (s *argoWorkFlowStoreImpl) FindByID(ctx context.Context, id int64) (WorkFlow ArgoWorkflow, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&WorkFlow).WhereAllWithDeleted().Where("id = ?", id).Scan(ctx, &WorkFlow)
	if err != nil {
		return
	}
	return
}

func (s *argoWorkFlowStoreImpl) FindByTaskID(ctx context.Context, id string) (*ArgoWorkflow, error) {
	var err error
	workFlow := &ArgoWorkflow{}
	q := s.db.Operator.Core.NewSelect().Model(workFlow).WhereAllWithDeleted()
	err = q.Where("task_id = ?", id).Scan(ctx, workFlow)
	if err != nil {
		return nil, err
	}
	return workFlow, nil
}

func (s *argoWorkFlowStoreImpl) FindByUsername(ctx context.Context, username string, taskType types.TaskType, per, page int) (WorkFlows []ArgoWorkflow, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&WorkFlows).
		ExcludeColumn("reason").
		Where("username = ?", username).Where("task_type = ?", taskType)

	query = query.Order("submit_time DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *argoWorkFlowStoreImpl) FindByUsernameWithTaskTypes(ctx context.Context, username string, taskTypes []types.TaskType, per, page int) (WorkFlows []ArgoWorkflow, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&WorkFlows).
		ExcludeColumn("reason").
		Where("username = ?", username).
		Where("task_type IN (?)", bun.In(taskTypes))

	query = query.Order("submit_time DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *argoWorkFlowStoreImpl) CreateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error) {
	wf, err := s.FindByTaskID(ctx, workFlow.TaskId)
	if err == nil && wf.ID != 0 {
		// already exists
		return wf, nil
	}
	res, err := s.db.Core.NewInsert().Model(&workFlow).Exec(ctx, &workFlow)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to save WorkFlow in db, error:%w", err)
	}

	return &workFlow, nil
}

// mainly for update status
func (s *argoWorkFlowStoreImpl) UpdateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error) {
	_, err := s.db.Core.NewUpdate().Model(&workFlow).WherePK().Exec(ctx)
	return &workFlow, err
}

// UpdateWorkFlowByTaskID
func (s *argoWorkFlowStoreImpl) UpdateWorkFlowByTaskID(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error) {
	_, err := s.db.Core.NewUpdate().Model(&workFlow).Where("task_id = ?", workFlow.TaskId).Exec(ctx)
	return &workFlow, err
}

// delete workflow by id
func (s *argoWorkFlowStoreImpl) DeleteWorkFlow(ctx context.Context, id int64) error {
	err := s.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		runSql := "update argo_workflows set status=? where id=? and status in (?)"
		_, err := tx.Exec(runSql,
			string(types.DFCancelled),
			id,
			bun.In([]string{
				string(v1alpha1.WorkflowUnknown),
				string(v1alpha1.WorkflowRunning),
				string(v1alpha1.WorkflowPending),
			}))
		if err != nil {
			return err
		}
		_, err = tx.NewDelete().Model(&ArgoWorkflow{}).Where("id = ?", id).Exec(ctx)
		return err
	})
	return err
}

// Status is v1alpha1.WorkflowRunning
func (s *argoWorkFlowStoreImpl) ListAllRunningEvaluations(ctx context.Context) (WorkFlows []ArgoWorkflow, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&WorkFlows).
		Where("status = ?", v1alpha1.WorkflowRunning).
		Scan(ctx)
	return
}

func (s *argoWorkFlowStoreImpl) ListRunningWorkflowsByUserUUID(ctx context.Context, userUUID string) (WorkFlows []ArgoWorkflow, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&WorkFlows).
		Where("user_uuid = ?", userUUID).
		Where("status in (?)", bun.In([]string{string(v1alpha1.WorkflowRunning), string(v1alpha1.WorkflowPending)})).
		Scan(ctx)
	return
}

func (s *argoWorkFlowStoreImpl) GetClusterWorkflows(ctx context.Context, req types.ClusterWFReq) ([]ArgoWorkflow, int, error) {
	var result []ArgoWorkflow
	query := s.db.Operator.Core.NewSelect().Model(&result).WhereAllWithDeleted()

	if req.ClusterID != "" {
		query = query.Where("cluster_id = ?", req.ClusterID)
	}
	if req.ClusterNode != "" {
		query = query.Where(
			"? = ANY(STRING_TO_ARRAY(COALESCE(cluster_node, ''), ','))",
			req.ClusterNode,
		)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.ResourceName != "" {
		query = query.Where("resource_name = ?", req.ResourceName)
	}
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		query = query.Where("task_name LIKE ? OR username LIKE ?", searchPattern, searchPattern)
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	query = query.Order("id DESC").Limit(req.Per).Offset((req.Page - 1) * req.Per)
	err = query.Scan(ctx, &result)
	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

func (s *argoWorkFlowStoreImpl) ListWorkflowsByTimeRange(ctx context.Context, req types.WorkflowTimeRangeReq) ([]ArgoWorkflow, int, error) {
	var result []ArgoWorkflow
	query := s.db.Operator.Core.NewSelect().Model(&result).WhereAllWithDeleted()

	if req.StartTime != nil {
		query = query.Where("submit_time >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		query = query.Where("submit_time <= ?", req.EndTime)
	}

	// Get total count
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if req.PageSize > 0 {
		query = query.Limit(req.PageSize)
	}
	if req.Page > 0 {
		query = query.Offset((req.Page - 1) * req.PageSize)
	}

	query = query.Order("submit_time DESC")
	err = query.Scan(ctx, &result)
	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}


func (s *argoWorkFlowStoreImpl) ListWorkflowsNeedingReconcile(ctx context.Context, statuses []v1alpha1.WorkflowPhase, timeoutMin int, limit int) ([]ArgoWorkflow, error) {
	var result []ArgoWorkflow
	timeoutAt := time.Now().Add(-time.Duration(timeoutMin) * time.Minute)

	// bun auto-filters soft-deleted records (WHERE deleted_at IS NULL)
	// because this query does NOT use WhereAllWithDeleted().
	err := s.db.Operator.Core.NewSelect().
		Model(&result).
		Where("status_update_at < ?", timeoutAt).
		Where("status IN (?)", bun.In(statuses)).
		Limit(limit).
		Order("status_update_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return result, nil
}
