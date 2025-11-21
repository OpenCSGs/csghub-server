package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentInstanceTask represents the link between an agent instance and an async task
type AgentInstanceTask struct {
	ID          int64               `bun:",pk,autoincrement" json:"id"`
	InstanceID  int64               `bun:",notnull" json:"instance_id"`
	TaskType    types.AgentTaskType `bun:",notnull" json:"task_type"` // Agent task type (e.g., "finetune", "inference")
	TaskID      string              `bun:",notnull" json:"task_id"`
	SessionUUID string              `bun:",notnull" json:"session_uuid"` // Session UUID
	UserUUID    string              `bun:",notnull" json:"user_uuid"`    // User UUID
	times
}

// AgentInstanceTaskStore provides database operations for AgentInstanceTask
type AgentInstanceTaskStore interface {
	Create(ctx context.Context, task *AgentInstanceTask) (*AgentInstanceTask, error)
}

// agentInstanceTaskStoreImpl is the implementation of AgentInstanceTaskStore
type agentInstanceTaskStoreImpl struct {
	db *DB
}

// NewAgentInstanceTaskStore creates a new AgentInstanceTaskStore
func NewAgentInstanceTaskStore() AgentInstanceTaskStore {
	return &agentInstanceTaskStoreImpl{
		db: defaultDB,
	}
}

// NewAgentInstanceTaskStoreWithDB creates a new AgentInstanceTaskStore with a specific DB
func NewAgentInstanceTaskStoreWithDB(db *DB) AgentInstanceTaskStore {
	return &agentInstanceTaskStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentInstanceTask into the database
func (s *agentInstanceTaskStoreImpl) Create(ctx context.Context, task *AgentInstanceTask) (*AgentInstanceTask, error) {
	res, err := s.db.Core.NewInsert().Model(task).Exec(ctx, task)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"instance_id": task.InstanceID,
			"task_id":     task.TaskID,
		})
	}
	return task, nil
}
