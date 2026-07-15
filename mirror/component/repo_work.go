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

// repoWorker handles repository mirror jobs registered on the mirror repo queue.
type repoWorker struct {
	river.WorkerDefaults[workhub.RepoArgs]
	mirrorTaskStore repoTaskStore
	syncer          repoSyncer
	lfsJobClient    database.MirrorLFSJobClient
}

// Timeout returns the per-job timeout for repository mirror jobs.
func (w *repoWorker) Timeout(*river.Job[workhub.RepoArgs]) time.Duration {
	return workhub.MirrorRepoJobTimeout
}

// repoTaskStore is the task state API needed by repository workhub jobs.
type repoTaskStore interface {
	FindByID(ctx context.Context, ID int64) (*database.MirrorTask, error)
	CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error)
	UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, statusAction string) (database.MirrorTask, error)
}

// repoSyncer performs the external Git repository mirror operation.
type repoSyncer interface {
	SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error)
}

// RepoWorkDeps contains dependencies supplied by the mirror package at worker initialization.
type RepoWorkDeps struct {
	// MirrorTaskStore updates task, repository, and mirror status transactionally.
	MirrorTaskStore repoTaskStore
	// Syncer executes the actual Git repository mirror operation.
	Syncer repoSyncer
	// LFSJobClient enqueues LFS work after repository sync finds LFS objects.
	LFSJobClient database.MirrorLFSJobClient
	// MaxWorkers controls the repository mirror queue concurrency.
	MaxWorkers int
}

// repoSlogArgs appends repo job fields and latest mirror information to slog args.
func repoSlogArgs(args workhub.RepoArgs, task *database.MirrorTask, attrs ...any) []any {
	attrs = append(attrs,
		slog.Int64("mirror_id", args.MirrorID),
		slog.Int64("repository_id", args.RepositoryID),
		slog.Int64("mirror_task_id", args.MirrorTaskID),
	)
	if task == nil || task.Mirror == nil {
		return attrs
	}
	attrs = append(attrs, slog.String("source_url", task.Mirror.SourceUrl))
	if task.Mirror.Repository != nil {
		attrs = append(attrs,
			slog.String("repo_type", string(task.Mirror.Repository.RepositoryType)),
			slog.String("repo_path", task.Mirror.Repository.Path),
		)
	}
	return attrs
}

// Work runs the repository mirror task.
func (w *repoWorker) Work(ctx context.Context, job *river.Job[workhub.RepoArgs]) (workErr error) {
	args := job.Args
	var task *database.MirrorTask
	slog.InfoContext(ctx, "working on repo job", repoSlogArgs(args, task,
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
			slog.ErrorContext(ctx, "repo job work exited", repoSlogArgs(args, task,
				slog.Bool("success", false),
				slog.String("error", originalErr.Error()),
				slog.Any("context", ctx.Err()),
				slog.Int64("job_id", job.ID),
				slog.Bool("snooze", snooze),
			)...)
			return
		}

		slog.InfoContext(ctx, "repo job work exited", repoSlogArgs(args, task,
			slog.Bool("success", true),
			slog.Int64("job_id", job.ID),
		)...)
	}()

	var err error
	task, err = w.mirrorTaskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find mirror task", repoSlogArgs(args, task, slog.String("error", err.Error()))...)
		workErr = fmt.Errorf("find mirror task: %w", err)
		return
	}
	slog.InfoContext(ctx, "loaded mirror repo task", repoSlogArgs(args, task, slog.String("task_status", string(task.Status)))...)
	if skip, reason := shouldSkipRepoJob(task, args); skip {
		slog.InfoContext(ctx, "skip stale mirror repo job", repoSlogArgs(args, task,
			slog.String("reason", reason),
			slog.String("task_status", string(task.Status)),
		)...)
		return
	}

	beforeStatus := task.Status
	task, err = w.prepareRepoTask(ctx, *task)
	if err != nil {
		workErr = err
		return
	}
	slog.InfoContext(ctx, "prepared mirror repo task", repoSlogArgs(args, task,
		slog.String("before_status", string(beforeStatus)),
		slog.String("after_status", string(task.Status)),
	)...)
	if task.Status != types.MirrorRepoSyncStart {
		slog.ErrorContext(ctx, "skip mirror repo job after prepare", repoSlogArgs(args, task,
			slog.String("task_status", string(task.Status)),
		)...)
		return
	}

	syncedTask, err := w.syncer.SyncRepo(ctx, task.Mirror, task)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "mirror repo sync canceled", repoSlogArgs(args, task, slog.String("error", err.Error()))...)
			workErr = err
			return
		}
		slog.ErrorContext(ctx, "failed to sync mirror repo task", repoSlogArgs(args, task, slog.String("error", err.Error()))...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorFail); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror repo task", repoSlogArgs(args, task, slog.String("error", updateErr.Error()))...)
			workErr = fmt.Errorf("mark repo sync failed: %w", updateErr)
			return
		}
		workErr = err
		return
	}
	if syncedTask == nil || syncedTask.Mirror == nil || syncedTask.Mirror.Repository == nil {
		workErr = fmt.Errorf("synced mirror repo task has no mirror repository")
		return
	}

	if _, err := w.mirrorTaskStore.CompleteRepoSyncAndInsertLFSJob(ctx, database.CompleteRepoSyncInput{
		Task:          *syncedTask,
		DefaultBranch: syncedTask.Mirror.Repository.DefaultBranch,
		JobClient:     w.lfsJobClient,
		JobInput: database.MirrorLFSJobInput{
			MirrorID:     syncedTask.MirrorID,
			RepositoryID: syncedTask.Mirror.RepositoryID,
			MirrorTaskID: syncedTask.ID,
			SourceURL:    syncedTask.Mirror.SourceUrl,
			Priority:     syncedTask.Priority,
		},
	}); err != nil {
		workErr = fmt.Errorf("enqueue mirror LFS job: %w", err)
		return
	}
	return
}

// NewRepoWorkClient creates a workhub worker client configured for repository
// mirror tasks.
func NewRepoWorkClient(ctx context.Context, databaseDSN string, deps RepoWorkDeps) (workhub.WorkClient, error) {
	if deps.MirrorTaskStore == nil {
		return nil, fmt.Errorf("mirror task store is required")
	}
	if deps.Syncer == nil {
		return nil, fmt.Errorf("repo syncer is required")
	}
	if deps.LFSJobClient == nil {
		return nil, fmt.Errorf("LFS job client is required")
	}
	config := newRepoRiverConfig(deps)

	return workhub.NewWorkClient(ctx, databaseDSN, config)
}

// newRepoRiverConfig builds the River config for repository mirror workers.
func newRepoRiverConfig(deps RepoWorkDeps) *river.Config {
	maxWorkers := deps.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	workers := workhub.NewWorkerRegistry(workhub.WorkerOverrides{
		MirrorRepo: &repoWorker{
			mirrorTaskStore: deps.MirrorTaskStore,
			syncer:          deps.Syncer,
			lfsJobClient:    deps.LFSJobClient,
		},
	})

	return &river.Config{
		Queues: map[string]river.QueueConfig{
			workhub.MirrorRepoQueue: {MaxWorkers: maxWorkers},
		},
		Workers: workers,
	}
}

// prepareRepoTask moves a queued or retryable repo task into the running state.
func (w *repoWorker) prepareRepoTask(ctx context.Context, task database.MirrorTask) (*database.MirrorTask, error) {
	switch task.Status {
	case types.MirrorRepoSyncFailed:
		retried, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorRetry)
		if err != nil {
			return nil, fmt.Errorf("retry mirror repo task: %w", err)
		}
		task = retried
	case types.MirrorQueued:
	default:
		return &task, nil
	}

	started, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
	if err != nil {
		return nil, fmt.Errorf("start mirror repo task: %w", err)
	}
	return &started, nil
}

// shouldSkipRepoJob reports whether a repo job no longer owns the current task and why.
func shouldSkipRepoJob(task *database.MirrorTask, args workhub.RepoArgs) (bool, string) {
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
	if isRepoJobTerminalStatus(task.Status) {
		return true, "terminal_status"
	}
	return false, ""
}

// isRepoJobTerminalStatus reports whether a repo workhub job has nothing left to do.
func isRepoJobTerminalStatus(status types.MirrorTaskStatus) bool {
	switch status {
	case types.MirrorRepoSyncFinished,
		types.MirrorRepoSyncFatal,
		types.MirrorLfsSyncStart,
		types.MirrorLfsSyncFinished,
		types.MirrorLfsSyncFailed,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsIncomplete,
		types.MirrorCanceled,
		types.MirrorRepoTooLarge:
		return true
	default:
		return false
	}
}
