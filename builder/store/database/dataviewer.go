package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type Dataviewer struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	RepoID     int64  `bun:",notnull" json:"repo_id"`
	RepoPath   string `bun:",notnull" json:"repo_path"`
	RepoBranch string `bun:",notnull" json:"repo_branch"`
	WorkflowID string `bun:",notnull" json:"workflow_id"`
	times
	DataviewerJob *DataviewerJob `bun:"rel:has-one,join:workflow_id=workflow_id" json:"dataviewer_job"`
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

type DataviewerStore interface {
	GetViewerByRepoID(ctx context.Context, repoID int64) (*Dataviewer, error)
	CreateViewer(ctx context.Context, viewer Dataviewer) error
	CreateJob(ctx context.Context, job DataviewerJob) error
	UpdateViewer(ctx context.Context, viewer Dataviewer) (*Dataviewer, error)
	GetJob(ctx context.Context, workflowID string) (*DataviewerJob, error)
	UpdateJob(ctx context.Context, job DataviewerJob) (*DataviewerJob, error)
	GetRunningJobsByRepoID(ctx context.Context, repoID int64) ([]DataviewerJob, error)
}

type dataviewerStoreImpl struct {
	db *DB
}

func NewDataviewerStore() DataviewerStore {
	return &dataviewerStoreImpl{db: defaultDB}
}

func NewDataViewerStoreWithDB(db *DB) DataviewerStore {
	return &dataviewerStoreImpl{db: db}
}

func (s *dataviewerStoreImpl) GetViewerByRepoID(ctx context.Context, repoID int64) (*Dataviewer, error) {
	var dataViewer Dataviewer
	err := s.db.Operator.Core.NewSelect().Model(&dataViewer).Relation("DataviewerJob").Where("dataviewer.repo_id = ?", repoID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select viewer by repo_id %d error: %w", repoID, err)
	}
	return &dataViewer, nil
}

func (s *dataviewerStoreImpl) CreateViewer(ctx context.Context, viewer Dataviewer) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		res, err := tx.NewInsert().Model(&viewer).Exec(ctx, &viewer)
		if err := assertAffectedOneRow(res, err); err != nil {
			return fmt.Errorf("create dataviewer repo_id %d, workflow_id %s error: %w", viewer.RepoID, viewer.WorkflowID, err)
		}

		job := DataviewerJob{
			RepoID:     viewer.RepoID,
			WorkflowID: viewer.WorkflowID,
			Status:     types.WorkflowPending,
			AutoCard:   true,
		}

		res, err = tx.NewInsert().Model(&job).Exec(ctx, &job)
		if err := assertAffectedOneRow(res, err); err != nil {
			return fmt.Errorf("create dataviewer job repo_id %d, workflow_id %s error: %w", viewer.RepoID, viewer.WorkflowID, err)
		}

		return nil
	})

	return err
}

func (s *dataviewerStoreImpl) CreateJob(ctx context.Context, job DataviewerJob) error {
	res, err := s.db.Operator.Core.NewInsert().Model(&job).Exec(ctx, &job)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create dataviewer job repo_id %d, workflow_id %s error: %w", job.RepoID, job.WorkflowID, err)
	}
	return nil
}

func (s *dataviewerStoreImpl) UpdateViewer(ctx context.Context, input Dataviewer) (*Dataviewer, error) {
	_, err := s.db.Operator.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update viewer by id %d error: %w", input.ID, err)
	}
	return &input, nil
}

func (s *dataviewerStoreImpl) GetJob(ctx context.Context, workflowID string) (*DataviewerJob, error) {
	var job DataviewerJob
	err := s.db.Operator.Core.NewSelect().Model(&job).Where("workflow_id = ?", workflowID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select viewer job by workflow_id %s error: %w", workflowID, err)
	}
	return &job, nil
}

func (s *dataviewerStoreImpl) UpdateJob(ctx context.Context, job DataviewerJob) (*DataviewerJob, error) {
	res, err := s.db.Operator.Core.NewUpdate().Model(&job).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update viewer job workflow_id %s error: %w", job.WorkflowID, err)
	}

	return &job, nil
}

func (s *dataviewerStoreImpl) GetRunningJobsByRepoID(ctx context.Context, repoID int64) ([]DataviewerJob, error) {
	var jobs []DataviewerJob
	_, err := s.db.Operator.Core.NewSelect().Model(&jobs).
		Where("repo_id = ?", repoID).
		Where("status >= 0 and status <=1").
		Exec(ctx, &jobs)
	if err != nil {
		return nil, fmt.Errorf("select running viewer jobs by repo_id %d, error: %w", repoID, err)
	}
	return jobs, nil
}
