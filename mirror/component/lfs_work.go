package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// lfsWorker handles Git LFS mirror jobs registered on the mirror LFS queue.
type lfsWorker struct {
	river.WorkerDefaults[workhub.LFSArgs]
	mirrorTaskStore lfsTaskStore
	syncer          lfsSyncer
	repoFilter      lfsRepoFilter
}

// Timeout returns the per-job timeout for Git LFS mirror jobs.
func (w *lfsWorker) Timeout(*river.Job[workhub.LFSArgs]) time.Duration {
	return workhub.MirrorLFSJobTimeout
}

// lfsTaskStore is the task state API needed by Git LFS workhub jobs.
type lfsTaskStore interface {
	FindByID(ctx context.Context, ID int64) (*database.MirrorTask, error)
	UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, statusAction string) (database.MirrorTask, error)
}

// lfsSyncer performs the external Git LFS mirror operation.
type lfsSyncer interface {
	SyncLFS(ctx context.Context, task *database.MirrorTask) error
}

// lfsRepoFilter decides whether a repository is allowed to sync LFS objects.
type lfsRepoFilter interface {
	ShouldSync(ctx context.Context, repoID int64) (bool, string, error)
}

// LFSWorkDeps contains dependencies supplied by the mirror package at worker initialization.
type LFSWorkDeps struct {
	// MirrorTaskStore updates task, repository, and mirror status transactionally.
	MirrorTaskStore lfsTaskStore
	// Syncer executes the actual Git LFS mirror operation.
	Syncer lfsSyncer
	// RepoFilter decides whether the repository is allowed to sync LFS objects.
	RepoFilter lfsRepoFilter
	// MaxWorkers controls the Git LFS mirror queue concurrency.
	MaxWorkers int
}

// lfsSlogArgs appends LFS job fields and latest mirror information to slog args.
func lfsSlogArgs(args workhub.LFSArgs, task *database.MirrorTask, attrs ...any) []any {
	attrs = append(attrs,
		slog.Int64("mirror_id", args.MirrorID),
		slog.Int64("repository_id", args.RepositoryID),
		slog.Int64("mirror_task_id", args.MirrorTaskID),
	)
	if task != nil && task.Mirror != nil {
		attrs = append(attrs, slog.String("source_url", task.Mirror.SourceUrl))
	}
	return attrs
}

// Work runs the LFS sync task.
func (w *lfsWorker) Work(ctx context.Context, job *river.Job[workhub.LFSArgs]) (workErr error) {
	args := job.Args
	var task *database.MirrorTask
	slog.InfoContext(ctx, "working on LFS job", lfsSlogArgs(args, task,
		slog.Int("attempts", job.Attempt),
		slog.Int("max_attempts", job.MaxAttempts),
		slog.Int("priority", job.Priority),
		slog.Int64("job_id", job.ID),
	)...)

	defer func() {
		if workErr != nil {
			originalErr := workErr
			snooze := errors.Is(ctx.Err(), context.DeadlineExceeded) && errors.Is(originalErr, context.DeadlineExceeded)
			if snooze {
				workErr = river.JobSnooze(0)
			}
			slog.ErrorContext(ctx, "LFS job work exited", lfsSlogArgs(args, task,
				slog.Bool("success", false),
				slog.String("error", originalErr.Error()),
				slog.Any("context", ctx.Err()),
				slog.Int64("job_id", job.ID),
				slog.Bool("snooze", snooze),
			)...)
			return
		}

		slog.InfoContext(ctx, "LFS job work exited", lfsSlogArgs(args, task,
			slog.Bool("success", true),
			slog.Int64("job_id", job.ID),
		)...)
	}()

	var err error
	task, err = w.mirrorTaskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find mirror LFS task", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
		workErr = fmt.Errorf("find mirror task: %w", err)
		return
	}
	slog.InfoContext(ctx, "loaded mirror LFS task", lfsSlogArgs(args, task, slog.String("task_status", string(task.Status)))...)
	if skip, reason := shouldSkipLFSJob(task, args); skip {
		slog.InfoContext(ctx, "skip stale mirror LFS job", lfsSlogArgs(args, task,
			slog.String("reason", reason),
			slog.String("task_status", string(task.Status)),
		)...)
		return nil
	}

	beforeStatus := task.Status
	task, err = w.prepareLFSTask(ctx, *task)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prepare mirror LFS task", lfsSlogArgs(args, task,
			slog.String("before_status", string(beforeStatus)),
			slog.String("error", err.Error()),
		)...)
		workErr = err
		return
	}
	slog.InfoContext(ctx, "prepared mirror LFS task", lfsSlogArgs(args, task,
		slog.String("before_status", string(beforeStatus)),
		slog.String("after_status", string(task.Status)),
	)...)
	if task.Status != types.MirrorLfsSyncStart {
		slog.ErrorContext(ctx, "skip mirror LFS job after prepare", lfsSlogArgs(args, task,
			slog.String("task_status", string(task.Status)),
		)...)
		return nil
	}

	shouldSync, reason, err := w.repoFilter.ShouldSync(ctx, task.Mirror.RepositoryID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "mirror LFS repo filter canceled", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
			workErr = err
			return
		}
		slog.ErrorContext(ctx, "failed to check mirror LFS repo filter", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorFail); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror LFS filter task", lfsSlogArgs(args, task, slog.String("error", updateErr.Error()))...)
			workErr = fmt.Errorf("mark LFS filter failed: %w", updateErr)
			return
		}
		workErr = err
		return
	}
	if !shouldSync {
		slog.InfoContext(ctx, "skip mirror LFS sync by repo filter", lfsSlogArgs(args, task, slog.String("reason", reason))...)
		if _, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorTooLarge); err != nil {
			slog.ErrorContext(ctx, "failed to update status of too large mirror LFS task", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
			workErr = fmt.Errorf("mark mirror repository too large: %w", err)
			return
		}
		return
	}
	slog.InfoContext(ctx, "start mirror LFS sync", lfsSlogArgs(args, task)...)

	if err := w.syncer.SyncLFS(ctx, task); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "mirror LFS sync canceled", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
			workErr = err
			return
		}
		action := database.MirrorFail
		slog.ErrorContext(ctx, "failed to sync mirror LFS task", lfsSlogArgs(args, task,
			slog.String("action", action),
			slog.String("error", err.Error()),
		)...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, action); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror LFS task", lfsSlogArgs(args, task, slog.String("error", updateErr.Error()))...)
			workErr = fmt.Errorf("mark LFS sync failed: %w", updateErr)
			return
		}
		workErr = err
		return
	}

	task.Progress = 100
	if _, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorSuccess); err != nil {
		slog.ErrorContext(ctx, "failed to finish mirror LFS task", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
		workErr = fmt.Errorf("finish LFS mirror task: %w", err)
		return
	}
	slog.InfoContext(ctx, "finished mirror LFS task", lfsSlogArgs(args, task, slog.Int("progress", task.Progress))...)
	return
}

// NewLFSWorkClient creates a workhub worker client configured for Git LFS sync
// tasks.
func NewLFSWorkClient(ctx context.Context, databaseDSN string, deps LFSWorkDeps) (workhub.WorkClient, error) {
	if deps.MirrorTaskStore == nil {
		return nil, fmt.Errorf("mirror task store is required")
	}
	if deps.Syncer == nil {
		return nil, fmt.Errorf("LFS syncer is required")
	}
	if deps.RepoFilter == nil {
		return nil, fmt.Errorf("LFS repo filter is required")
	}
	config := newLFSRiverConfig(deps)

	return workhub.NewWorkClient(ctx, databaseDSN, config)
}

// newLFSRiverConfig builds the River config for Git LFS mirror workers.
func newLFSRiverConfig(deps LFSWorkDeps) *river.Config {
	maxWorkers := deps.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	workers := workhub.NewWorkerRegistry(workhub.WorkerOverrides{
		MirrorLFS: &lfsWorker{
			mirrorTaskStore: deps.MirrorTaskStore,
			syncer:          deps.Syncer,
			repoFilter:      deps.RepoFilter,
		},
	})

	return &river.Config{
		Queues: map[string]river.QueueConfig{
			workhub.MirrorLFSQueue: {MaxWorkers: maxWorkers},
		},
		Workers: workers,
	}
}

// prepareLFSTask moves a repo-synced task into the LFS running state.
func (w *lfsWorker) prepareLFSTask(ctx context.Context, task database.MirrorTask) (*database.MirrorTask, error) {
	switch task.Status {
	case types.MirrorLfsSyncFailed:
		retried, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorRetry)
		if err != nil {
			return nil, fmt.Errorf("retry mirror LFS task: %w", err)
		}
		task = retried
	case types.MirrorRepoSyncFinished:
	default:
		return &task, nil
	}
	started, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
	if err != nil {
		return nil, fmt.Errorf("start mirror LFS task: %w", err)
	}
	return &started, nil
}

// shouldSkipLFSJob reports whether an LFS job no longer owns the current task and why.
func shouldSkipLFSJob(task *database.MirrorTask, args workhub.LFSArgs) (bool, string) {
	if task == nil || task.Mirror == nil {
		return false, ""
	}
	if args.MirrorID != 0 && task.MirrorID != args.MirrorID {
		return true, "mirror_id_mismatch"
	}
	if args.RepositoryID != 0 && task.Mirror.RepositoryID != args.RepositoryID {
		return true, "repository_id_mismatch"
	}
	if task.Mirror.CurrentTaskID != 0 && task.Mirror.CurrentTaskID != task.ID {
		return true, "stale_current_task"
	}
	if isLFSJobTerminalStatus(task.Status) {
		return true, "terminal_status"
	}
	return false, ""
}

// isLFSJobTerminalStatus reports whether a Git LFS workhub job has nothing left to do.
func isLFSJobTerminalStatus(status types.MirrorTaskStatus) bool {
	switch status {
	case types.MirrorLfsSyncFinished,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsIncomplete,
		types.MirrorCanceled,
		types.MirrorRepoTooLarge,
		types.MirrorRepoSyncFatal:
		return true
	default:
		return false
	}
}
