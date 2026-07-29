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
	mirrorTaskStore mirrorTaskStore
	syncer          lfsSyncer
	urgentManager   *workhub.UrgentManager
}

// Timeout returns the per-job timeout for Git LFS mirror jobs.
func (w *lfsWorker) Timeout(*river.Job[workhub.LFSArgs]) time.Duration {
	return workhub.MirrorLFSJobTimeout
}

// lfsSyncer performs the external Git LFS mirror operation.
type lfsSyncer interface {
	SyncLFS(ctx context.Context, task *database.MirrorTask) error
}

// LFSWorkDeps contains dependencies supplied by the mirror package at worker initialization.
type LFSWorkDeps struct {
	// MirrorTaskStore updates task, repository, and mirror status transactionally.
	MirrorTaskStore mirrorTaskStore
	// Syncer executes the actual Git LFS mirror operation.
	Syncer lfsSyncer
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
func (w *lfsWorker) Work(ctx context.Context, job *river.Job[workhub.LFSArgs]) error {
	return runMirrorWork(ctx, job, mirrorWorkConfig[workhub.LFSArgs]{
		name:             "LFS",
		manager:          w.urgentManager,
		preemptionDelay:  urgentJobDelay,
		isUrgent:         func(args workhub.LFSArgs) bool { return args.Urgent },
		expectedQueue:    workhub.LFSQueue,
		validateQueue:    workhub.ValidateLFSQueue,
		logArgs:          lfsSlogArgs,
		work:             w.work,
		failureTarget:    mirrorLFSJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(w.mirrorTaskStore),
	})
}

// work executes the LFS mirror business flow and returns the latest task for lifecycle logging.
func (w *lfsWorker) work(ctx context.Context, args workhub.LFSArgs, retryCount int) (*database.MirrorTask, error) {
	task, err := w.mirrorTaskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find mirror LFS task", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
		return task, fmt.Errorf("find mirror task: %w", err)
	}
	slog.InfoContext(ctx, "loaded mirror LFS task", lfsSlogArgs(args, task, slog.String("task_status", string(task.Status)))...)
	if skip, reason := shouldSkipLFSJob(task, args); skip {
		slog.InfoContext(ctx, "skip stale mirror LFS job", lfsSlogArgs(args, task,
			slog.String("reason", reason),
			slog.String("task_status", string(task.Status)),
		)...)
		return task, nil
	}
	if args.Urgent && w.urgentManager != nil {
		done, err := beginUrgentWork(w.urgentManager, ctx)
		if err != nil {
			return task, err
		}
		defer done()
	}
	task.RetryCount = retryCount

	beforeStatus := task.Status
	task, err = w.prepareLFSTask(ctx, *task)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prepare mirror LFS task", lfsSlogArgs(args, task,
			slog.String("before_status", string(beforeStatus)),
			slog.String("error", err.Error()),
		)...)
		return task, err
	}
	slog.InfoContext(ctx, "prepared mirror LFS task", lfsSlogArgs(args, task,
		slog.String("before_status", string(beforeStatus)),
		slog.String("after_status", string(task.Status)),
	)...)
	if task.Status != types.MirrorLfsSyncStart {
		slog.ErrorContext(ctx, "skip mirror LFS job after prepare", lfsSlogArgs(args, task,
			slog.String("task_status", string(task.Status)),
		)...)
		return task, nil
	}

	slog.InfoContext(ctx, "start mirror LFS sync", lfsSlogArgs(args, task)...)

	if err := w.syncer.SyncLFS(ctx, task); err != nil {
		if isUrgentWorkCancellation(ctx, err) || errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "mirror LFS sync canceled", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
			return task, err
		}
		action := database.MirrorFail
		slog.ErrorContext(ctx, "failed to sync mirror LFS task", lfsSlogArgs(args, task,
			slog.String("action", action),
			slog.String("error", err.Error()),
		)...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, action); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror LFS task", lfsSlogArgs(args, task, slog.String("error", updateErr.Error()))...)
			return task, fmt.Errorf("mark LFS sync failed: %w", updateErr)
		}
		return task, err
	}

	if err := contextCauseError(ctx); err != nil {
		return task, err
	}
	task.Progress = 100
	if _, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorSuccess); err != nil {
		slog.ErrorContext(ctx, "failed to finish mirror LFS task", lfsSlogArgs(args, task, slog.String("error", err.Error()))...)
		return task, fmt.Errorf("finish LFS mirror task: %w", err)
	}
	slog.InfoContext(ctx, "finished mirror LFS task", lfsSlogArgs(args, task, slog.Int("progress", task.Progress))...)
	return task, nil
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
	worker := newLFSWorker(deps)
	config := newLFSRiverConfigForWorker(deps, worker)
	client, err := workhub.NewWorkClient(ctx, databaseDSN, config)
	if err != nil {
		return nil, err
	}
	manager := client.ConfigureUrgentManager(workhub.UrgentManagerConfig{
		NormalQueue:       workhub.MirrorLFSQueue,
		NormalQueueConfig: config.Queues[workhub.MirrorLFSQueue],
		UrgentIdleDelay:   urgentIdleDelay,
	})
	worker.urgentManager = manager
	return client, nil
}

// newLFSWorker builds the LFS worker shared by normal and urgent queues.
func newLFSWorker(deps LFSWorkDeps) *lfsWorker {
	return &lfsWorker{
		mirrorTaskStore: deps.MirrorTaskStore,
		syncer:          deps.Syncer,
	}
}

// newLFSRiverConfig builds the River config for Git LFS mirror workers.
func newLFSRiverConfig(deps LFSWorkDeps) *river.Config {
	return newLFSRiverConfigForWorker(deps, newLFSWorker(deps))
}

// newLFSRiverConfigForWorker registers one worker instance for normal and urgent LFS queues.
func newLFSRiverConfigForWorker(deps LFSWorkDeps, worker *lfsWorker) *river.Config {
	maxWorkers := deps.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	workers := workhub.NewWorkerRegistry(workhub.WorkerOverrides{
		MirrorLFS: worker,
	})

	return &river.Config{
		ErrorHandler: newMirrorJobErrorHandler(deps.MirrorTaskStore),
		Queues: map[string]river.QueueConfig{
			workhub.MirrorLFSQueue:       {MaxWorkers: maxWorkers},
			workhub.MirrorLFSUrgentQueue: {MaxWorkers: workhub.UrgentMaxWorkers(maxWorkers)},
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
