package migrations

import (
	"context"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type ArgoWorkflow struct {
	ID          int64                  `bun:",pk,autoincrement" json:"id"`
	Username    string                 `bun:",notnull" json:"username"`
	UserUUID    string                 `bun:",notnull" json:"user_uuid"`
	TaskName    string                 `bun:",notnull" json:"task_name"` // user input name
	TaskId      string                 `bun:",notnull" json:"task_id"`   // generated task id
	TaskType    types.TaskType         `bun:",notnull" json:"task_type"`
	RepoIds     []string               `bun:",notnull,type:jsonb" json:"repo_ids"`
	RepoType    string                 `bun:",notnull" json:"repo_type"`
	TaskDesc    string                 `bun:"," json:"task_desc"`
	Status      v1alpha1.WorkflowPhase `bun:"," json:"status"`
	Reason      string                 `bun:"," json:"reason"`       // reason for status
	Image       string                 `bun:",notnull" json:"image"` // ArgoWorkFlow framework
	Datasets    []string               `bun:",notnull,type:jsonb" json:"datasets"`
	ResourceId  int64                  `bun:",nullzero" json:"resource_id"`
	SubmitTime  time.Time              `bun:",nullzero,notnull,default:current_timestamp" json:"submit_time"`
	StartTime   time.Time              `bun:",nullzero" json:"start_time"`
	EndTime     time.Time              `bun:",nullzero" json:"end_time"`
	ResultURL   string                 `bun:",nullzero" json:"result_url"`
	DownloadURL string                 `bun:",nullzero" json:"download_url"`
	FailuresURL string                 `bun:",nullzero" json:"failures_url"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, ArgoWorkflow{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*ArgoWorkflow)(nil)).
			Index("idx_workflow_user_uuid").
			Column("username", "task_id").
			Exec(ctx)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ArgoWorkflow{})
	})
}
