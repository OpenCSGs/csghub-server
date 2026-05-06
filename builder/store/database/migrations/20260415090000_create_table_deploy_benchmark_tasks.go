package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type DeployBenchmarkTask struct {
	bun.BaseModel      `bun:"table:deploy_benchmark_tasks"`
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
	Hardware           string     `bun:"type:jsonb,notnull,default:'{}'" json:"hardware"`
	RequestTemplate    string     `bun:"type:jsonb,notnull,default:'{}'" json:"request_template"`
	BenchmarkConfig    string     `bun:"type:jsonb,notnull,default:'{}'" json:"benchmark_config"`
	ResultSummary      string     `bun:"type:jsonb,notnull,default:'{}'" json:"result_summary"`
	RawResult          string     `bun:"type:jsonb,notnull,default:'{}'" json:"raw_result"`
	ErrorMessage       string     `bun:",type:text,nullzero" json:"error_message"`
	StartedAt          *time.Time `bun:",nullzero" json:"started_at,omitempty"`
	FinishedAt         *time.Time `bun:",nullzero" json:"finished_at,omitempty"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, &DeployBenchmarkTask{}); err != nil {
			return err
		}

		indexes := []struct {
			name    string
			columns []string
			unique  bool
		}{
			{name: "idx_deploy_benchmark_tasks_deploy_id_created_at", columns: []string{"deploy_id", "created_at"}},
			{name: "idx_deploy_benchmark_tasks_status_created_at", columns: []string{"status", "created_at"}},
			{name: "idx_deploy_benchmark_tasks_workflow_id", columns: []string{"workflow_id"}},
			{name: "idx_deploy_benchmark_tasks_user_uuid_created_at", columns: []string{"user_uuid", "created_at"}},
			{name: "uq_deploy_benchmark_tasks_trigger", columns: []string{"deploy_id", "trigger_source", "trigger_key"}, unique: true},
		}

		for _, idx := range indexes {
			query := db.NewCreateIndex().Model(&DeployBenchmarkTask{}).Index(idx.name).Column(idx.columns...).IfNotExists()
			if idx.unique {
				query = query.Unique()
			}
			if _, err := query.Exec(ctx); err != nil {
				return err
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &DeployBenchmarkTask{})
	})
}
