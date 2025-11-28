package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AgentInstanceTask struct {
	ID          int64               `bun:",pk,autoincrement" json:"id"`
	InstanceID  int64               `bun:",notnull" json:"instance_id"`
	TaskType    types.AgentTaskType `bun:",notnull" json:"task_type"` // Agent task type (e.g., "finetune", "inference")
	TaskID      string              `bun:",notnull" json:"task_id"`
	SessionUUID string              `bun:",notnull" json:"session_uuid"` // Session UUID
	UserUUID    string              `bun:",notnull" json:"user_uuid"`    // User UUID
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentInstanceTask{})
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, "ALTER TABLE agent_instance_tasks ADD CONSTRAINT unique_agent_instance_task_instance_type_task UNIQUE (instance_id, task_type, task_id)")
		if err != nil {
			return fmt.Errorf("add constraint unique_agent_instance_task_instance_type_task fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &AgentInstanceTask{})
	})
}
