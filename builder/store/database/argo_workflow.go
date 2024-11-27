package database

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/common/types"
)

type ArgoWorkFlowStore struct {
	db *DB
}

func NewArgoWorkFlowStore() *ArgoWorkFlowStore {
	return &ArgoWorkFlowStore{
		db: defaultDB,
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
	SubmitTime   time.Time              `bun:",nullzero,notnull,default:current_timestamp" json:"submit_time"`
	StartTime    time.Time              `bun:",nullzero" json:"start_time"`
	EndTime      time.Time              `bun:",nullzero" json:"end_time"`
	ResultURL    string                 `bun:"," json:"result_url"`
	DownloadURL  string                 `bun:"," json:"download_url"`
	FailuresURL  string                 `bun:"," json:"failures_url"`
}

func (s *ArgoWorkFlowStore) FindByID(ctx context.Context, id int64) (WorkFlow ArgoWorkflow, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&WorkFlow).Where("id = ?", id).Scan(ctx, &WorkFlow)
	if err != nil {
		return
	}
	return
}

func (s *ArgoWorkFlowStore) FindByTaskID(ctx context.Context, id string) (WorkFlow ArgoWorkflow, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&WorkFlow).Where("task_id = ?", id).Scan(ctx, &WorkFlow)
	if err != nil {
		return
	}
	return
}

func (s *ArgoWorkFlowStore) FindByUsername(ctx context.Context, username string, per, page int) (WorkFlows []ArgoWorkflow, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&WorkFlows).
		Where("username = ?", username)

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

func (s *ArgoWorkFlowStore) CreateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error) {
	res, err := s.db.Core.NewInsert().Model(&workFlow).Exec(ctx, &workFlow)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to save WorkFlow in db, error:%w", err)
	}

	return &workFlow, nil
}

// mainly for update status
func (s *ArgoWorkFlowStore) UpdateWorkFlow(ctx context.Context, workFlow ArgoWorkflow) (*ArgoWorkflow, error) {
	_, err := s.db.Core.NewUpdate().Model(&workFlow).WherePK().Exec(ctx)
	return &workFlow, err
}

// delete workflow by id
func (s *ArgoWorkFlowStore) DeleteWorkFlow(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model(&ArgoWorkflow{}).Where("id = ?", id).Exec(ctx)
	return err
}
