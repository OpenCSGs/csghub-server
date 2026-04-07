package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// AgentInstanceScheduler represents the scheduler configuration (parent table).
type AgentInstanceScheduler struct {
	bun.BaseModel  `bun:"table:agent_instance_schedulers"`
	ID             int64      `bun:",pk,autoincrement" json:"id"`
	UserUUID       string     `bun:",notnull" json:"user_uuid"`
	InstanceID     int64      `bun:",notnull" json:"instance_id"`
	Name           string     `bun:",notnull" json:"name"`
	Prompt         string     `bun:",type:text,notnull" json:"prompt"`
	ScheduleType   string     `bun:",notnull" json:"schedule_type"`
	CronExpression string     `bun:",notnull" json:"cron_expression"`
	StartDate      time.Time  `bun:",notnull" json:"start_date"`
	StartTime      time.Time  `bun:",notnull" json:"start_time"`
	EndDate        *time.Time `bun:",nullzero" json:"end_date,omitempty"`
	ScheduleID     string     `bun:",nullzero" json:"schedule_id,omitempty"`
	Status         string     `bun:",notnull,default:'active'" json:"status"`
	LastRunAt      *time.Time `bun:",nullzero" json:"last_run_at,omitempty"`
	times
}

// AgentInstanceSchedulerTask represents a single execution of a scheduler (child table).
type AgentInstanceSchedulerTask struct {
	bun.BaseModel `bun:"table:agent_instance_scheduler_tasks"`
	ID            int64      `bun:",pk,autoincrement" json:"id"`
	SchedulerID   int64      `bun:",notnull" json:"scheduler_id"`
	InstanceID    int64      `bun:",notnull" json:"instance_id"`
	UserUUID      string     `bun:",notnull" json:"user_uuid"`
	Name          string     `bun:",notnull" json:"name"`
	WorkflowID    string     `bun:",nullzero" json:"workflow_id,omitempty"`
	SessionUUID   string     `bun:",nullzero" json:"session_uuid,omitempty"`
	Status        string     `bun:",notnull" json:"status"`
	ErrorMessage  string     `bun:",type:text,nullzero" json:"error_message,omitempty"`
	StartedAt     time.Time  `bun:",notnull" json:"started_at"`
	CompletedAt   *time.Time `bun:",nullzero" json:"completed_at,omitempty"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentInstanceScheduler{}, &AgentInstanceSchedulerTask{})
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().Model(&AgentInstanceScheduler{}).
			Index("idx_agent_schedulers_user_uuid_instance_id").
			Column("user_uuid", "instance_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().Model(&AgentInstanceScheduler{}).
			Index("idx_agent_schedulers_status").
			Column("status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		// Index for search by name (case-insensitive)
		_, err = db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_agent_schedulers_user_uuid_lower_name ON agent_instance_schedulers (user_uuid, LOWER(name))")
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().Model(&AgentInstanceSchedulerTask{}).
			Index("idx_agent_scheduler_tasks_scheduler_id").
			Column("scheduler_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		// Index for ListByInstanceID: instance_id + user_uuid (query filters by instance first)
		_, err = db.NewCreateIndex().Model(&AgentInstanceSchedulerTask{}).
			Index("idx_agent_scheduler_tasks_instance_id_user_uuid").
			Column("instance_id", "user_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		// Index for search by name within instance scope
		_, err = db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_agent_scheduler_tasks_instance_user_lower_name ON agent_instance_scheduler_tasks (instance_id, user_uuid, LOWER(name))")
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().Model(&AgentInstanceSchedulerTask{}).
			Index("idx_agent_scheduler_tasks_workflow_id").
			Column("workflow_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().Model(&AgentInstanceSchedulerTask{}).
			Index("idx_agent_scheduler_tasks_status").
			Column("status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		// Add scheduler_quota_per_user to the existing instance config
		_, err = db.ExecContext(ctx, `
			UPDATE agent_configs
			SET config = config || '{"scheduler_quota_per_user": 5}'::jsonb,
			    updated_at = NOW()
			WHERE name = 'instance'
		`)
		if err != nil {
			return err
		}

		// Add "scheduler" capability to the system/genius-agent built-in instance
		_, err = db.ExecContext(ctx, `
			UPDATE agent_instances
			SET metadata = jsonb_set(
				metadata,
				'{capabilities}',
				COALESCE(metadata->'capabilities', '[]'::jsonb) || '"scheduler"'::jsonb
			),
			    updated_at = NOW()
			WHERE type = 'code' AND content_id = 'system/genius-agent' AND built_in = true
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Remove "scheduler" capability from system/genius-agent built-in instance
		_, err := db.ExecContext(ctx, `
			UPDATE agent_instances
			SET metadata = jsonb_set(
				metadata,
				'{capabilities}',
				COALESCE(
					(SELECT jsonb_agg(elem)
					 FROM jsonb_array_elements(metadata->'capabilities') elem
					 WHERE elem::text != '"scheduler"'),
					'[]'::jsonb
				)
			),
			    updated_at = NOW()
			WHERE type = 'code' AND content_id = 'system/genius-agent' AND built_in = true
		`)
		if err != nil {
			return err
		}

		// Remove scheduler_quota_per_user from instance config
		_, err = db.ExecContext(ctx, `
			UPDATE agent_configs
			SET config = config - 'scheduler_quota_per_user',
			    updated_at = NOW()
			WHERE name = 'instance'
		`)
		if err != nil {
			return err
		}
		// Drop child table first, then parent
		return dropTables(ctx, db, &AgentInstanceSchedulerTask{}, &AgentInstanceScheduler{})
	})
}
