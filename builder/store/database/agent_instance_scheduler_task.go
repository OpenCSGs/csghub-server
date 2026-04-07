package database

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentInstanceSchedulerTask struct {
	ID           int64      `bun:",pk,autoincrement" json:"id"`
	SchedulerID  int64      `bun:",notnull" json:"scheduler_id"`
	InstanceID   int64      `bun:",notnull" json:"instance_id"`
	UserUUID     string     `bun:",notnull" json:"user_uuid"`
	Name         string     `bun:",notnull" json:"name"`
	WorkflowID   string     `bun:",nullzero" json:"workflow_id,omitempty"`
	SessionUUID  string     `bun:",nullzero" json:"session_uuid,omitempty"`
	Status       string     `bun:",notnull" json:"status"` // running, success, failed
	ErrorMessage string     `bun:",type:text,nullzero" json:"error_message,omitempty"`
	StartedAt    time.Time  `bun:",notnull" json:"started_at"`
	CompletedAt  *time.Time `bun:",nullzero" json:"completed_at,omitempty"`
	times
}

type AgentInstanceSchedulerTaskStore interface {
	Create(ctx context.Context, task *AgentInstanceSchedulerTask) (*AgentInstanceSchedulerTask, error)
	FindByID(ctx context.Context, id int64) (*AgentInstanceSchedulerTask, error)
	Update(ctx context.Context, task *AgentInstanceSchedulerTask) error
	ListByInstanceID(ctx context.Context, userUUID string, instanceID int64, filter types.AgentSchedulerTaskFilter, per, page int) ([]AgentInstanceSchedulerTask, int, error)
}

type agentInstanceSchedulerTaskStoreImpl struct {
	db *DB
}

func NewAgentInstanceSchedulerTaskStore() AgentInstanceSchedulerTaskStore {
	return &agentInstanceSchedulerTaskStoreImpl{
		db: defaultDB,
	}
}

func NewAgentInstanceSchedulerTaskStoreWithDB(db *DB) AgentInstanceSchedulerTaskStore {
	return &agentInstanceSchedulerTaskStoreImpl{
		db: db,
	}
}

func (s *agentInstanceSchedulerTaskStoreImpl) Create(ctx context.Context, task *AgentInstanceSchedulerTask) (*AgentInstanceSchedulerTask, error) {
	res, err := s.db.Core.NewInsert().Model(task).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"scheduler_id": task.SchedulerID,
		})
	}
	return task, nil
}

func (s *agentInstanceSchedulerTaskStoreImpl) Update(ctx context.Context, task *AgentInstanceSchedulerTask) error {
	res, err := s.db.Core.NewUpdate().Model(task).Where("id = ?", task.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"task_id": task.ID,
		})
	}
	return nil
}

func (s *agentInstanceSchedulerTaskStoreImpl) FindByID(ctx context.Context, id int64) (*AgentInstanceSchedulerTask, error) {
	task := &AgentInstanceSchedulerTask{}
	err := s.db.Core.NewSelect().Model(task).Where("id = ?", id).Scan(ctx, task)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"task_id": id,
		})
	}
	return task, nil
}

const taskDefaultOrder = "updated_at DESC"

// applyTaskFilters applies scheduler_id, status, and search (name) filters to the query.
func (s *agentInstanceSchedulerTaskStoreImpl) applyTaskFilters(query *bun.SelectQuery, filter types.AgentSchedulerTaskFilter) *bun.SelectQuery {
	if filter.SchedulerID != nil {
		query = query.Where("scheduler_id = ?", *filter.SchedulerID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Search != "" {
		filter.Search = strings.TrimSpace(filter.Search)
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?)", searchPattern)
	}
	return query
}

func (s *agentInstanceSchedulerTaskStoreImpl) ListByInstanceID(ctx context.Context, userUUID string, instanceID int64, filter types.AgentSchedulerTaskFilter, per, page int) ([]AgentInstanceSchedulerTask, int, error) {
	var tasks []AgentInstanceSchedulerTask
	query := s.db.Core.NewSelect().Model(&tasks).
		Where("instance_id = ?", instanceID).
		Where("user_uuid = ?", userUUID)
	query = s.applyTaskFilters(query, filter)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"instance_id": instanceID,
		})
	}

	err = query.Order(taskDefaultOrder).Limit(per).Offset((page - 1) * per).Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"instance_id": instanceID,
		})
	}

	return tasks, total, nil
}
