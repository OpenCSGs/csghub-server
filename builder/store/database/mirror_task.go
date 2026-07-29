package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/looplab/fsm"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type mirrorTaskStoreImpl struct {
	db *DB
}

type MirrorTaskStore interface {
	Create(ctx context.Context, task MirrorTask) (MirrorTask, error)
	Update(ctx context.Context, task MirrorTask) (MirrorTask, error)
	UpdateProgress(ctx context.Context, task MirrorTask) (MirrorTask, error)
	UpdateStatusAndRepoSyncStatus(ctx context.Context, task MirrorTask, statusAction string) (MirrorTask, error)
	FindByMirrorID(ctx context.Context, mirrorID int64) (*MirrorTask, error)
	Delete(ctx context.Context, ID int64) error
	SetMirrorCurrentTaskID(ctx context.Context, task MirrorTask) error
	FindByID(ctx context.Context, ID int64) (*MirrorTask, error)
	ListByStatusWithPriority(ctx context.Context, status []types.MirrorTaskStatus, per, page int) ([]MirrorTask, error)
}

// MirrorTaskJobStore extends MirrorTaskStore with workhub job transaction helpers.
type MirrorTaskJobStore interface {
	MirrorTaskStore
	// CancelMirrorTaskByIDWithJobCancel cancels business state and workhub jobs in one transaction.
	CancelMirrorTaskByIDWithJobCancel(ctx context.Context, taskID int64, jobCancelClient MirrorJobCancelClient) (bool, error)
	CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input CompleteRepoSyncInput) (MirrorTask, error)
	RequeueMirrorRepoTask(ctx context.Context, input RequeueMirrorRepoTaskInput) (MirrorTask, error)
	UpdateCommitCheckpoint(ctx context.Context, taskID int64, beforeCommitID, afterCommitID string) (MirrorTask, error)
}

// MirrorLFSJobClient inserts Git LFS mirror jobs in the same transaction as task state updates.
type MirrorLFSJobClient interface {
	// InsertMirrorLFSJobTx inserts one Git LFS mirror job in the provided transaction.
	InsertMirrorLFSJobTx(ctx context.Context, tx *sql.Tx, input MirrorLFSJobInput) (int64, error)
}

// MirrorJobCancelClient cancels workhub jobs in the same transaction as task state updates.
type MirrorJobCancelClient interface {
	// JobCancelTx cancels one workhub job in the provided transaction.
	JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error
}

// MirrorLFSJobInput describes the Git LFS job queued after repo sync finds LFS objects.
type MirrorLFSJobInput struct {
	// MirrorID identifies the mirror record that owns the LFS sync.
	MirrorID int64
	// RepositoryID identifies the local repository being synchronized.
	RepositoryID int64
	// MirrorTaskID identifies the mirror task shared by repo and LFS jobs.
	MirrorTaskID int64
	// SourceURL is the upstream Git URL that owns the LFS objects.
	SourceURL string
	// Priority controls LFS job scheduling order.
	Priority types.MirrorPriority
	// Urgent routes the job to the urgent LFS queue.
	Urgent bool
}

// CompleteRepoSyncInput carries the database updates that make repo sync completion atomic with LFS job creation.
type CompleteRepoSyncInput struct {
	// Task is the mirror task that just finished repository sync.
	Task MirrorTask
	// DefaultBranch is the synchronized default branch reported by Git server.
	DefaultBranch string
	// JobClient inserts the follow-up LFS job inside the same transaction.
	JobClient MirrorLFSJobClient
	// JobInput describes the follow-up LFS job.
	JobInput MirrorLFSJobInput
}

// RequeueMirrorRepoTaskInput carries the data needed to manually enqueue a repository mirror sync.
type RequeueMirrorRepoTaskInput struct {
	// MirrorID identifies the mirror that should be synchronized again.
	MirrorID int64
	// RepositoryID identifies the local repository bound to the mirror.
	RepositoryID int64
	// Username updates the source Git username when provided; nil preserves the stored value.
	Username *string
	// AccessToken updates the source Git access token when provided; nil preserves the stored value.
	AccessToken *string
	// Priority controls the new repo job scheduling order.
	Priority types.MirrorPriority
	// Urgent routes the new repo job to the urgent queue.
	Urgent bool
	// JobClient inserts the repo workhub job inside the same transaction.
	JobClient MirrorJobClient
	// JobCancelClient cancels replaced workhub jobs inside the same transaction.
	JobCancelClient MirrorJobCancelClient
}

var mirrorTaskStatusToRepoStatusMap = map[types.MirrorTaskStatus]types.RepositorySyncStatus{
	types.MirrorQueued:           types.SyncStatusPending,
	types.MirrorRepoSyncStart:    types.SyncStatusInProgress,
	types.MirrorRepoSyncFailed:   types.SyncStatusFailed,
	types.MirrorRepoSyncFinished: types.SyncStatusInProgress,
	types.MirrorRepoSyncFatal:    types.SyncStatusFailed,
	types.MirrorLfsSyncStart:     types.SyncStatusInProgress,
	types.MirrorLfsSyncFailed:    types.SyncStatusFailed,
	types.MirrorLfsSyncFinished:  types.SyncStatusCompleted,
	types.MirrorLfsSyncFatal:     types.SyncStatusFailed,
	types.MirrorLfsIncomplete:    types.SyncStatusFailed,
	types.MirrorCanceled:         types.SyncStatusCanceled,
	types.MirrorRepoTooLarge:     types.SyncStatusFailed,
}

func NewMirrorTaskStore() MirrorTaskStore {
	return &mirrorTaskStoreImpl{
		db: defaultDB,
	}
}

func NewMirrorTaskStoreWithDB(db *DB) MirrorTaskStore {
	return &mirrorTaskStoreImpl{
		db: db,
	}
}

// NewMirrorTaskJobStore creates a mirror task store with workhub job transaction helpers.
func NewMirrorTaskJobStore() MirrorTaskJobStore {
	return &mirrorTaskStoreImpl{
		db: defaultDB,
	}
}

// NewMirrorTaskJobStoreWithDB creates a mirror task job store using the provided database.
func NewMirrorTaskJobStoreWithDB(db *DB) MirrorTaskJobStore {
	return &mirrorTaskStoreImpl{
		db: db,
	}
}

type MirrorTask struct {
	ID                 int64                  `bun:",pk,autoincrement" json:"id"`
	MirrorID           int64                  `bun:",notnull" json:"mirror_id"`
	Mirror             *Mirror                `bun:"rel:belongs-to,join:mirror_id=id" json:"mirror"`
	ErrorMessage       string                 `bun:",nullzero" json:"error_message"`
	Status             types.MirrorTaskStatus `bun:",notnull" json:"status"`
	RetryCount         int                    `bun:",notnull,default:0" json:"retry_count"`
	Payload            string                 `bun:"," json:"payload"`
	Priority           types.MirrorPriority   `bun:",notnull" json:"priority"`
	BeforeLastCommitID string                 `bun:"" json:"before_last_commit_id"`
	AfterLastCommitID  string                 `bun:"" json:"after_last_commit_id"`
	// RepoJobID stores the River job ID for the repository sync phase.
	RepoJobID int64 `bun:",nullzero" json:"repo_job_id"`
	// LFSJobID stores the River job ID for the Git LFS sync phase.
	LFSJobID int64 `bun:",nullzero" json:"lfs_job_id"`
	Progress int   `bun:"" json:"progress"`
	// IsUrgent reports whether this task was submitted through the urgent queues.
	IsUrgent bool `bun:",notnull,default:false" json:"is_urgent"`

	StartedAt  time.Time `bun:",nullzero"`
	FinishedAt time.Time `bun:",nullzero"`
	times
}

type MirrorTaskWithFSM struct {
	mirrorTask *MirrorTask
	from       types.MirrorTaskStatus
	fsm        *fsm.FSM
}

const (
	MirrorContinue    = "continue"
	MirrorFail        = "fail"
	MirrorSuccess     = "success"
	MirrorRetry       = "retry"
	MirrorCancel      = "cancel"
	MirrorFatal       = "fatal"
	MirrorNoLfsToSync = "no_lfs_to_sync"
	MirrorTooLarge    = "too_large"
)

func NewMirrorTaskWithFSM(mt *MirrorTask) MirrorTaskWithFSM {
	return MirrorTaskWithFSM{
		mirrorTask: mt,
		from:       mt.Status,
		fsm: fsm.NewFSM(
			string(mt.Status),
			fsm.Events{
				{
					Name: MirrorContinue,
					Src: []string{
						string(types.MirrorQueued),
					},
					Dst: string(types.MirrorRepoSyncStart),
				},
				{
					Name: MirrorRetry,
					Src: []string{
						string(types.MirrorRepoSyncFailed),
					},
					Dst: string(types.MirrorQueued),
				},
				{
					Name: MirrorRetry,
					Src: []string{
						string(types.MirrorLfsSyncFailed),
					},
					Dst: string(types.MirrorRepoSyncFinished),
				},
				{
					Name: MirrorNoLfsToSync,
					Src: []string{
						string(types.MirrorRepoSyncStart),
					},
					Dst: string(types.MirrorLfsSyncFinished),
				},
				{
					Name: MirrorContinue,
					Src: []string{
						string(types.MirrorRepoSyncFinished),
					},
					Dst: string(types.MirrorLfsSyncStart),
				},
				{
					Name: MirrorFail,
					Src: []string{
						string(types.MirrorRepoSyncStart),
					},
					Dst: string(types.MirrorRepoSyncFailed),
				},
				{
					Name: MirrorFail,
					Src: []string{
						string(types.MirrorLfsSyncStart),
					},
					Dst: string(types.MirrorLfsSyncFailed),
				},
				{
					Name: MirrorFatal,
					Src: []string{
						string(types.MirrorRepoSyncFailed),
					},
					Dst: string(types.MirrorRepoSyncFatal),
				},
				{
					Name: MirrorFatal,
					Src: []string{
						string(types.MirrorLfsSyncFailed),
					},
					Dst: string(types.MirrorLfsSyncFatal),
				},
				{
					Name: MirrorSuccess,
					Src: []string{
						string(types.MirrorRepoSyncStart),
					},
					Dst: string(types.MirrorRepoSyncFinished),
				},
				{
					Name: MirrorSuccess,
					Src: []string{
						string(types.MirrorLfsSyncStart),
					},
					Dst: string(types.MirrorLfsSyncFinished),
				},
				{
					Name: MirrorCancel,
					Src: []string{
						string(types.MirrorQueued),
						string(types.MirrorRepoSyncStart),
						string(types.MirrorRepoSyncFailed),
						string(types.MirrorRepoSyncFinished),
						string(types.MirrorRepoSyncFatal),
						string(types.MirrorLfsSyncStart),
						string(types.MirrorLfsSyncFailed),
						string(types.MirrorLfsSyncFatal),
					},
					Dst: string(types.MirrorCanceled),
				},
				{
					Name: MirrorTooLarge,
					Src: []string{
						string(types.MirrorLfsSyncStart),
					},
					Dst: string(types.MirrorRepoTooLarge),
				},
			},
			fsm.Callbacks{
				"entry_state": func(ctx context.Context, event *fsm.Event) {
					mt.Status = types.MirrorTaskStatus(event.Dst)
				},
			},
		),
	}
}

func (m *MirrorTaskWithFSM) SubmitEvent(ctx context.Context, event string) bool {
	return m.fsm.Event(ctx, event) == nil
}

func (m *MirrorTaskWithFSM) Current() string {
	return m.fsm.Current()
}

func (m *mirrorTaskStoreImpl) Create(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	err := m.db.Operator.Core.NewInsert().Model(&task).Scan(ctx, &task)
	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) Update(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	_, err := m.db.Operator.Core.NewUpdate().Model(&task).WherePK().Exec(ctx)
	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) UpdateProgress(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	_, err := m.db.Operator.Core.NewUpdate().
		Model(&task).
		Column("progress", "error_message").
		WherePK().
		Exec(ctx)
	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) FindByMirrorID(ctx context.Context, mirrorID int64) (*MirrorTask, error) {
	var task MirrorTask
	err := m.db.Operator.Core.NewSelect().Model(&task).Where("mirror_id = ?", mirrorID).Scan(ctx)
	return &task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) Delete(ctx context.Context, ID int64) error {
	var task MirrorTask
	task.ID = ID
	_, err := m.db.Operator.Core.NewDelete().Model(&task).WherePK().Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) SetMirrorCurrentTaskID(ctx context.Context, task MirrorTask) error {
	var mirror Mirror
	mirror.ID = task.MirrorID
	mirror.CurrentTaskID = task.ID
	_, err := m.db.Operator.Core.NewUpdate().Model(&mirror).WherePK().Column("current_task_id").Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) FindByID(ctx context.Context, ID int64) (*MirrorTask, error) {
	var task MirrorTask
	task.ID = ID
	err := m.db.Operator.Core.NewSelect().
		Model(&task).
		Relation("Mirror").
		Relation("Mirror.Repository").
		Where("mirror_task.id = ?", ID).
		Scan(ctx)
	return &task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) ListByStatusWithPriority(ctx context.Context, status []types.MirrorTaskStatus, per, page int) ([]MirrorTask, error) {
	var tasks []MirrorTask
	err := m.db.Operator.Core.NewSelect().
		Model(&tasks).
		Relation("Mirror").
		Relation("Mirror.Repository").
		Where("mirror_task.status IN (?)", bun.In(status)).
		OrderExpr("mirror_task.priority ASC, mirror_task.updated_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	return tasks, errorx.HandleDBError(err, nil)
}

// RequeueMirrorRepoTask cancels active work for one mirror and enqueues a fresh repo sync job atomically.
func (m *mirrorTaskStoreImpl) RequeueMirrorRepoTask(ctx context.Context, input RequeueMirrorRepoTaskInput) (MirrorTask, error) {
	if input.MirrorID == 0 {
		return MirrorTask{}, fmt.Errorf("mirror id is required")
	}
	if input.JobClient == nil {
		return MirrorTask{}, fmt.Errorf("mirror repo job client is required")
	}
	if (input.Username == nil) != (input.AccessToken == nil) {
		return MirrorTask{}, fmt.Errorf("mirror username and access token must be updated together")
	}
	if input.Priority == 0 {
		input.Priority = types.MediumMirrorPriority
	}

	var task MirrorTask
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var mirror Mirror
		if err := tx.NewSelect().
			Model(&mirror).
			Where("id = ?", input.MirrorID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return fmt.Errorf("failed to lock mirror: %w", err)
		}
		if input.RepositoryID != 0 && mirror.RepositoryID != input.RepositoryID {
			return fmt.Errorf("mirror repository mismatch, mirror repository id: %d, input repository id: %d", mirror.RepositoryID, input.RepositoryID)
		}
		if input.Username != nil {
			mirror.Username = *input.Username
			mirror.AccessToken = *input.AccessToken
		}

		var repo Repository
		if err := tx.NewSelect().
			Model(&repo).
			Where("id = ?", mirror.RepositoryID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return fmt.Errorf("failed to lock repository: %w", err)
		}
		mirror.Repository = &repo

		var oldTasks []MirrorTask
		cancelStatuses := requeueCancelableMirrorTaskStatuses()
		if err := tx.NewSelect().
			Model(&oldTasks).
			Where("mirror_id = ?", mirror.ID).
			Where("status IN (?)", bun.In(cancelStatuses)).
			For("UPDATE OF mirror_task").
			Scan(ctx); err != nil {
			return fmt.Errorf("failed to lock old mirror tasks: %w", err)
		}
		now := time.Now()
		for _, oldTask := range oldTasks {
			if err := cancelMirrorTaskJobsTx(ctx, tx.Tx, oldTask, input.JobCancelClient); err != nil {
				return err
			}
		}
		if len(oldTasks) > 0 {
			if _, err := tx.NewUpdate().
				Model((*MirrorTask)(nil)).
				Set("status = ?", types.MirrorCanceled).
				Set("updated_at = ?", now).
				Set("finished_at = ?", now).
				Where("mirror_id = ?", mirror.ID).
				Where("status IN (?)", bun.In(cancelStatuses)).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to cancel old mirror tasks: %w", err)
			}
		}

		task = MirrorTask{
			MirrorID: mirror.ID,
			Mirror:   &mirror,
			Priority: input.Priority,
			Status:   types.MirrorQueued,
			IsUrgent: input.Urgent,
		}
		if err := tx.NewInsert().Model(&task).Scan(ctx, &task); err != nil {
			return fmt.Errorf("failed to create mirror task: %w", err)
		}

		if err := updateRepoSyncStatus(ctx, tx, repo.ID, types.SyncStatusPending); err != nil {
			return fmt.Errorf("failed to update repository sync status: %w", err)
		}
		mirrorUpdate := tx.NewUpdate().
			Model((*Mirror)(nil)).
			Set("status = ?", types.MirrorQueued).
			Set("mirror_priority = ?", input.Priority).
			Set("current_task_id = ?", task.ID).
			Set("updated_at = ?", now).
			Where("id = ?", mirror.ID)
		if input.Username != nil {
			mirrorUpdate = mirrorUpdate.
				Set("username = ?", mirror.Username).
				Set("access_token = ?", mirror.AccessToken)
		}
		if _, err := mirrorUpdate.Exec(ctx); err != nil {
			return fmt.Errorf("failed to update mirror status: %w", err)
		}

		repoJobID, err := input.JobClient.InsertMirrorRepoJobTx(ctx, tx.Tx, MirrorJobInput{
			MirrorID:     mirror.ID,
			RepositoryID: repo.ID,
			MirrorTaskID: task.ID,
			RepoType:     repo.RepositoryType,
			SourceURL:    mirror.SourceUrl,
			RepoPath:     repo.Path,
			Priority:     input.Priority,
			Urgent:       input.Urgent,
		})
		if err != nil {
			return fmt.Errorf("failed to insert mirror repo job: %w", err)
		}
		task.RepoJobID = repoJobID
		if _, err := tx.NewUpdate().
			Model(&task).
			Column("repo_job_id").
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to update mirror repo job id: %w", err)
		}
		task.Mirror = &mirror
		task.Mirror.Status = types.MirrorQueued
		task.Mirror.Priority = input.Priority
		task.Mirror.CurrentTaskID = task.ID
		task.Mirror.Repository.SyncStatus = types.SyncStatusPending
		return nil
	})
	return task, errorx.HandleDBError(err, nil)
}

// requeueCancelableMirrorTaskStatuses returns task states replaced by a manual re-sync.
func requeueCancelableMirrorTaskStatuses() []types.MirrorTaskStatus {
	return []types.MirrorTaskStatus{
		types.MirrorQueued,
		types.MirrorRepoSyncStart,
		types.MirrorRepoSyncFailed,
		types.MirrorRepoSyncFinished,
		types.MirrorLfsSyncStart,
		types.MirrorLfsSyncFailed,
	}
}

// CancelMirrorTaskByIDWithJobCancel cancels task, mirror, repository, and River jobs atomically.
func (m *mirrorTaskStoreImpl) CancelMirrorTaskByIDWithJobCancel(ctx context.Context, ID int64, jobCancelClient MirrorJobCancelClient) (bool, error) {
	var cancelled bool
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var task MirrorTask
		err := tx.NewSelect().
			Model(&task).
			Where("id = ?", ID).
			For("UPDATE OF mirror_task").
			Scan(ctx)
		if err != nil {
			return err
		}

		tFSM := NewMirrorTaskWithFSM(&task)
		if tFSM.SubmitEvent(ctx, MirrorCancel) {
			task.Status = types.MirrorTaskStatus(tFSM.Current())
			task.UpdatedAt = time.Now()
			task.FinishedAt = task.UpdatedAt
			_, err = tx.NewUpdate().
				Model(&task).
				Column("status", "updated_at", "finished_at").
				WherePK().
				Exec(ctx)
			if err != nil {
				return err
			}

			if err := cancelMirrorTaskJobsTx(ctx, tx.Tx, task, jobCancelClient); err != nil {
				return err
			}

			var mirror Mirror
			if err := tx.NewSelect().
				Model(&mirror).
				Where("id = ?", task.MirrorID).
				For("UPDATE").
				Scan(ctx); err != nil {
				return err
			}
			if mirror.CurrentTaskID == 0 || mirror.CurrentTaskID == task.ID {
				if err := updateMirrorAndRepoStateTx(ctx, tx, mirror, task.ID, task.Status); err != nil {
					return err
				}
			}
			cancelled = true
		}
		return nil
	})
	if err != nil {
		return false, errorx.HandleDBError(err, nil)
	}
	return cancelled, nil
}

// cancelMirrorTaskJobsTx cancels queued River jobs while the business cancel transaction is open.
func cancelMirrorTaskJobsTx(ctx context.Context, tx *sql.Tx, task MirrorTask, jobCancelClient MirrorJobCancelClient) error {
	if jobCancelClient == nil {
		return nil
	}
	if task.RepoJobID != 0 {
		if err := jobCancelClient.JobCancelTx(ctx, tx, task.RepoJobID); err != nil {
			return fmt.Errorf("failed to cancel mirror repo job %d: %w", task.RepoJobID, err)
		}
	}
	if task.LFSJobID != 0 {
		if err := jobCancelClient.JobCancelTx(ctx, tx, task.LFSJobID); err != nil {
			return fmt.Errorf("failed to cancel mirror LFS job %d: %w", task.LFSJobID, err)
		}
	}
	return nil
}

func (m *mirrorTaskStoreImpl) UpdateStatusAndRepoSyncStatus(ctx context.Context, task MirrorTask, statusAction string) (MirrorTask, error) {
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		task, err = updateMirrorTaskStateTx(ctx, tx, task, statusAction)
		if err != nil {
			return err
		}
		return nil
	})
	return task, errorx.HandleDBError(err, nil)
}

// CompleteRepoSyncAndInsertLFSJob commits repository sync results and the follow-up LFS job atomically.
func (m *mirrorTaskStoreImpl) CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input CompleteRepoSyncInput) (MirrorTask, error) {
	task := input.Task
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		task, err = updateMirrorTaskStateTx(ctx, tx, task, MirrorSuccess)
		if err != nil {
			return err
		}
		if input.DefaultBranch != "" {
			if _, err := tx.NewUpdate().
				Model((*Repository)(nil)).
				Set("default_branch = ?", input.DefaultBranch).
				Where("id = ?", task.Mirror.RepositoryID).
				Exec(ctx); err != nil {
				return err
			}
			if task.Mirror.Repository != nil {
				task.Mirror.Repository.DefaultBranch = input.DefaultBranch
			}
		}
		if _, err := tx.NewUpdate().
			Model((*Mirror)(nil)).
			Set("last_updated_at = ?", time.Now()).
			Where("id = ?", task.MirrorID).
			Exec(ctx); err != nil {
			return err
		}
		if input.JobClient == nil {
			return fmt.Errorf("mirror LFS job client is required")
		}
		jobInput := input.JobInput
		if jobInput.MirrorID == 0 {
			jobInput.MirrorID = task.MirrorID
		}
		if jobInput.RepositoryID == 0 && task.Mirror != nil {
			jobInput.RepositoryID = task.Mirror.RepositoryID
		}
		if jobInput.MirrorTaskID == 0 {
			jobInput.MirrorTaskID = task.ID
		}
		if jobInput.SourceURL == "" && task.Mirror != nil {
			jobInput.SourceURL = task.Mirror.SourceUrl
		}
		if jobInput.Priority == 0 {
			jobInput.Priority = task.Priority
		}
		lfsJobID, err := input.JobClient.InsertMirrorLFSJobTx(ctx, tx.Tx, jobInput)
		if err != nil {
			return fmt.Errorf("failed to insert mirror LFS job: %w", err)
		}
		task.LFSJobID = lfsJobID
		if _, err := tx.NewUpdate().
			Model(&task).
			Column("lfs_job_id").
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to update mirror LFS job id: %w", err)
		}
		return nil
	})
	return task, errorx.HandleDBError(err, nil)
}

// UpdateCommitCheckpoint persists repo sync commit checkpoints so retried jobs keep the original before commit.
func (m *mirrorTaskStoreImpl) UpdateCommitCheckpoint(ctx context.Context, taskID int64, beforeCommitID, afterCommitID string) (MirrorTask, error) {
	var task MirrorTask
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().
			Model(&task).
			Where("id = ?", taskID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return err
		}
		if beforeCommitID != "" {
			task.BeforeLastCommitID = beforeCommitID
		}
		if afterCommitID != "" {
			task.AfterLastCommitID = afterCommitID
		}
		task.UpdatedAt = time.Now()
		_, err := tx.NewUpdate().
			Model(&task).
			Column("before_last_commit_id", "after_last_commit_id", "updated_at").
			WherePK().
			Exec(ctx)
		return err
	})
	return task, errorx.HandleDBError(err, nil)
}

// updateMirrorTaskStateTx applies one FSM transition and keeps task, mirror, and repository status consistent.
func updateMirrorTaskStateTx(ctx context.Context, tx bun.Tx, task MirrorTask, statusAction string) (MirrorTask, error) {
	var current MirrorTask
	if err := tx.NewSelect().
		Model(&current).
		Where("mirror_task.id = ?", task.ID).
		For("UPDATE OF mirror_task").
		Scan(ctx); err != nil {
		return task, err
	}

	var mirror Mirror
	if err := tx.NewSelect().
		Model(&mirror).
		Where("mirror.id = ?", current.MirrorID).
		For("UPDATE").
		Scan(ctx); err != nil {
		return task, err
	}
	if task.Mirror != nil && task.Mirror.Repository != nil && task.Mirror.Repository.ID == mirror.RepositoryID {
		mirror.Repository = task.Mirror.Repository
	} else {
		var repo Repository
		if err := tx.NewSelect().
			Model(&repo).
			Where("id = ?", mirror.RepositoryID).
			Scan(ctx); err != nil {
			return task, err
		}
		mirror.Repository = &repo
	}
	if mirror.CurrentTaskID != 0 && mirror.CurrentTaskID != current.ID {
		return task, fmt.Errorf("mirror current task changed, current_task_id: %d, task_id: %d", mirror.CurrentTaskID, current.ID)
	}
	current.Mirror = &mirror

	tFSM := NewMirrorTaskWithFSM(&current)
	if !tFSM.SubmitEvent(ctx, statusAction) {
		return task, fmt.Errorf("mirror task status %s not allow action %s", current.Status, statusAction)
	}

	current.Status = types.MirrorTaskStatus(tFSM.Current())
	current.ErrorMessage = task.ErrorMessage
	if shouldClearMirrorTaskError(statusAction) {
		current.ErrorMessage = ""
	}
	current.Progress = task.Progress
	current.RetryCount = task.RetryCount
	current.BeforeLastCommitID = task.BeforeLastCommitID
	current.AfterLastCommitID = task.AfterLastCommitID
	current.UpdatedAt = time.Now()
	if isMirrorTaskRunningStatus(current.Status) && current.StartedAt.IsZero() {
		current.StartedAt = current.UpdatedAt
	}
	if isMirrorTaskTerminalStatus(current.Status) {
		current.FinishedAt = current.UpdatedAt
	}

	// Only update fields owned by task execution so concurrent priority changes are preserved.
	if _, err := tx.NewUpdate().
		Model(&current).
		Column("status", "error_message", "progress", "updated_at", "retry_count", "before_last_commit_id", "after_last_commit_id", "started_at", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return task, err
	}

	if current.Mirror == nil {
		return task, fmt.Errorf("mirror task %d has no mirror relation", current.ID)
	}
	if err := updateMirrorAndRepoStateTx(ctx, tx, *current.Mirror, current.ID, current.Status); err != nil {
		return task, err
	}

	current.Mirror.CurrentTaskID = current.ID
	current.Mirror.Status = current.Status
	return current, nil
}

// shouldClearMirrorTaskError reports whether a non-error transition should drop stale errors.
func shouldClearMirrorTaskError(statusAction string) bool {
	switch statusAction {
	case MirrorContinue, MirrorRetry, MirrorSuccess, MirrorNoLfsToSync:
		return true
	default:
		return false
	}
}

// updateMirrorAndRepoStateTx keeps the mirror and repository status aligned with one task state.
func updateMirrorAndRepoStateTx(ctx context.Context, tx bun.Tx, mirror Mirror, taskID int64, status types.MirrorTaskStatus) error {
	syncStatus, ok := mirrorTaskStatusToRepoStatusMap[status]
	if !ok {
		return fmt.Errorf("mirror task status %s has no repository sync status", status)
	}
	if _, err := tx.NewUpdate().
		Model(&Repository{}).
		Set("sync_status = ?", syncStatus).
		Where("id = ?", mirror.RepositoryID).
		Exec(ctx); err != nil {
		return err
	}

	if _, err := tx.NewUpdate().
		Model(&Mirror{}).
		Set("status = ?", status).
		Set("current_task_id = ?", taskID).
		Where("id = ?", mirror.ID).
		Exec(ctx); err != nil {
		return err
	}
	return nil
}

// isMirrorTaskRunningStatus reports whether a task status represents active execution.
func isMirrorTaskRunningStatus(status types.MirrorTaskStatus) bool {
	return status == types.MirrorRepoSyncStart || status == types.MirrorLfsSyncStart
}

// isMirrorTaskTerminalStatus reports whether no further work should run for the task.
func isMirrorTaskTerminalStatus(status types.MirrorTaskStatus) bool {
	switch status {
	case types.MirrorRepoSyncFatal,
		types.MirrorLfsSyncFinished,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsIncomplete,
		types.MirrorCanceled,
		types.MirrorRepoTooLarge:
		return true
	default:
		return false
	}
}
