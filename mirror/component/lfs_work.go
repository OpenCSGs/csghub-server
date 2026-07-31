package component

import (
	"context"
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

// Work runs the LFS sync task.
func (w *lfsWorker) Work(ctx context.Context, job *river.Job[workhub.LFSArgs]) error {
	return runWorkWithLog(ctx, job, mirrorWorkConfig[workhub.LFSArgs]{
		manager:   w.urgentManager,
		urgent:    job.Args.Urgent,
		args:      job.Args.MirrorArgs,
		stage:     "lfs",
		taskStore: w.mirrorTaskStore,
		work: func(ctx context.Context, args workhub.LFSArgs, retryCount int) (*database.MirrorTask, error) {
			return w.work(ctx, args, retryCount, job.ID)
		},
	})
}

// work executes the LFS mirror business flow and returns the latest task for lifecycle logging.
func (w *lfsWorker) work(ctx context.Context, args workhub.LFSArgs, retryCount int, jobID int64) (*database.MirrorTask, error) {
	task, err := w.mirrorTaskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find mirror LFS task",
			mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", err.Error()))...)
		return task, fmt.Errorf("find mirror task: %w", err)
	}
	slog.InfoContext(ctx, "loaded mirror LFS task", mirrorTaskSlogArgs(args.MirrorArgs, task)...)
	skip, err := checkLFSTaskInfo(task, args, jobID)
	if err != nil {
		slog.ErrorContext(ctx, "invalid mirror LFS task information",
			mirrorTaskSlogArgs(args.MirrorArgs, task,
				slog.Int64("job_id", jobID),
				slog.String("error", err.Error()),
			)...)
		return task, fmt.Errorf("check mirror LFS task information: %w", err)
	}
	if skip {
		slog.InfoContext(ctx, "skip mirror LFS task",
			mirrorTaskSlogArgs(args.MirrorArgs, task,
				slog.Int64("job_id", jobID),
			)...)
		return task, nil
	}
	task.RetryCount = retryCount

	beforeStatus := task.Status
	task, err = w.prepareLFSTask(ctx, *task)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prepare mirror LFS task",
			mirrorTaskSlogArgs(args.MirrorArgs, task,
				slog.String("before_status", string(beforeStatus)),
				slog.String("error", err.Error()),
			)...)
		return task, err
	}
	slog.InfoContext(ctx, "prepared mirror LFS task",
		mirrorTaskSlogArgs(args.MirrorArgs, task,
			slog.String("before_status", string(beforeStatus)),
			slog.String("after_status", string(task.Status)),
		)...)

	slog.InfoContext(ctx, "start mirror LFS sync", mirrorTaskSlogArgs(args.MirrorArgs, task)...)

	if err := w.syncer.SyncLFS(ctx, task); err != nil {
		if isWorkContextTermination(ctx, err) {
			slog.InfoContext(ctx, "mirror LFS sync stopped by execution context",
				mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", err.Error()))...)
			return task, err
		}
		action := database.MirrorFail
		slog.ErrorContext(ctx, "failed to sync mirror LFS task", mirrorTaskSlogArgs(args.MirrorArgs, task,
			slog.String("action", action),
			slog.String("error", err.Error()),
		)...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, action); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror LFS task",
				mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", updateErr.Error()))...)
			return task, fmt.Errorf("mark LFS sync failed: %w", updateErr)
		}
		return task, err
	}

	if err := ctx.Err(); err != nil {
		return task, err
	}
	task.Progress = 100
	if _, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorSuccess); err != nil {
		slog.ErrorContext(ctx, "failed to finish mirror LFS task",
			mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", err.Error()))...)
		return task, fmt.Errorf("finish LFS mirror task: %w", err)
	}
	slog.InfoContext(ctx, "finished mirror LFS task",
		mirrorTaskSlogArgs(args.MirrorArgs, task, slog.Int("progress", task.Progress))...)
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
	case types.MirrorLfsSyncStart:
		return &task, nil
	case types.MirrorLfsSyncFailed:
		retried, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorRetry)
		if err != nil {
			return &task, fmt.Errorf("retry mirror LFS task: %w", err)
		}
		if retried.Status != types.MirrorRepoSyncFinished {
			return &retried, fmt.Errorf("retried mirror LFS task has unexpected status %s", retried.Status)
		}
		task = retried
	case types.MirrorRepoSyncFinished:
	default:
		return &task, fmt.Errorf("cannot prepare mirror LFS task with status %s", task.Status)
	}
	started, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
	if err != nil {
		return &task, fmt.Errorf("start mirror LFS task: %w", err)
	}
	if started.Status != types.MirrorLfsSyncStart {
		return &started, fmt.Errorf("started mirror LFS task has unexpected status %s", started.Status)
	}
	return &started, nil
}

// checkLFSTaskInfo validates LFS job ownership and reports terminal tasks that no longer need work.
func checkLFSTaskInfo(task *database.MirrorTask, args workhub.LFSArgs, jobID int64) (bool, error) {
	if err := checkMirrorTaskInfo(task, args.MirrorArgs); err != nil {
		return false, err
	}
	if task.LFSJobID != jobID {
		return false, fmt.Errorf("LFS job ID mismatch: task=%d job=%d", task.LFSJobID, jobID)
	}
	switch task.Status {
	case types.MirrorRepoSyncFinished, types.MirrorLfsSyncStart, types.MirrorLfsSyncFailed:
		return false, nil
	case types.MirrorCanceled,
		types.MirrorRepoTooLarge,
		types.MirrorLfsIncomplete,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsSyncFinished:
		return true, nil
	default:
		return false, fmt.Errorf("mirror LFS task status %s is not executable", task.Status)
	}
}
