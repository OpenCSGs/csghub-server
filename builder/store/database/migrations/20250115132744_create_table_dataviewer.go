package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type Dataviewer struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	RepoID     int64  `bun:",notnull" json:"repo_id"`
	RepoPath   string `bun:",notnull" json:"repo_path"`
	RepoBranch string `bun:",notnull" json:"repo_branch"`
	WorkflowID string `bun:",notnull" json:"workflow_id"`
	times
}

type DataviewerJob struct {
	ID         int64     `bun:",pk,autoincrement" json:"id"`
	RepoID     int64     `bun:",notnull" json:"repo_id"`
	WorkflowID string    `bun:",notnull" json:"workflow_id"`
	Status     int       `bun:",notnull" json:"status"`
	AutoCard   bool      `bun:",notnull" json:"auto_card"`
	CardData   string    `bun:",nullzero" json:"card_data"`
	CardMD5    string    `bun:",nullzero" json:"card_md5"`
	RunID      string    `bun:",nullzero" json:"run_id"`
	Logs       string    `bun:",nullzero" json:"logs"`
	StartTime  time.Time `bun:",nullzero" json:"start_time"`
	EndTime    time.Time `bun:",nullzero" json:"end_time"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Dataviewer{})
		if err != nil {
			return fmt.Errorf("create table dataviewers fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*Dataviewer)(nil)).
			Index("idx_unique_dataviewer_repoid").
			Column("repo_id").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_unique_dataviewer_repoid fail: %w", err)
		}

		err = createTables(ctx, db, DataviewerJob{})
		if err != nil {
			return fmt.Errorf("create table dataviewer_jobs fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*DataviewerJob)(nil)).
			Index("idx_unique_dataviewer_job_workflowid").
			Column("workflow_id").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_unique_dataviewer_job_workflowid fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*DataviewerJob)(nil)).
			Index("idx_dataviewer_job_repoid_status_workflow_updatetime").
			Column("repo_id", "status", "updated_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_dataviewer_job_repoid_status_workflow_updatetime fail: %w", err)
		}
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Dataviewer{}, DataviewerJob{})
	})
}
