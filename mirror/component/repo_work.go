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
	"opencsg.com/csghub-server/mirror/reposyncer"
)

// repoWorker handles repository mirror jobs registered on the mirror repo queue.
type repoWorker struct {
	river.WorkerDefaults[workhub.RepoArgs]
	mirrorTaskStore repoTaskStore
	syncer          repoSyncer
	lfsJobClient    database.MirrorLFSJobClient
	urgentManager   *workhub.UrgentManager
}

// Timeout returns the per-job timeout for repository mirror jobs.
func (w *repoWorker) Timeout(*river.Job[workhub.RepoArgs]) time.Duration {
	return workhub.MirrorRepoJobTimeout
}

// repoTaskStore is the task state API needed by repository workhub jobs.
type repoTaskStore interface {
	mirrorTaskStore
	CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error)
}

// repoSyncer performs the external Git repository mirror operation.
type repoSyncer interface {
	SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (reposyncer.RepoSyncResult, error)
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

// Work runs the repository mirror task.
func (w *repoWorker) Work(ctx context.Context, job *river.Job[workhub.RepoArgs]) error {
	return runWorkWithLog(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
		manager:   w.urgentManager,
		urgent:    job.Args.Urgent,
		args:      job.Args.MirrorArgs,
		stage:     "repo",
		taskStore: w.mirrorTaskStore,
		work: func(ctx context.Context, args workhub.RepoArgs, retryCount int) (*database.MirrorTask, error) {
			return w.work(ctx, args, retryCount, job.ID)
		},
	})
}

// work executes the repository mirror business flow and returns the latest task for lifecycle logging.
func (w *repoWorker) work(ctx context.Context, args workhub.RepoArgs, retryCount int, jobID int64) (*database.MirrorTask, error) {
	task, err := w.mirrorTaskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find mirror task",
			mirrorTaskSlogArgs(args.MirrorArgs, task,
				slog.String("error", err.Error()),
			)...)
		return task, fmt.Errorf("find mirror task: %w", err)
	}
	slog.InfoContext(ctx, "loaded mirror repo task", mirrorTaskSlogArgs(args.MirrorArgs, task)...)
	skip, err := checkRepoTaskInfo(task, args, jobID)
	if err != nil {
		slog.ErrorContext(ctx, "invalid mirror repo task information",
			mirrorTaskSlogArgs(args.MirrorArgs, task,
				slog.Int64("job_id", jobID),
				slog.String("error", err.Error()),
			)...)
		return task, fmt.Errorf("check mirror repo task information: %w", err)
	}
	if skip {
		slog.InfoContext(ctx, "skip mirror repo task",
			mirrorTaskSlogArgs(args.MirrorArgs, task, slog.Int64("job_id", jobID))...)
		return task, nil
	}
	task.RetryCount = retryCount

	beforeStatus := task.Status
	task, err = w.prepareRepoTask(ctx, *task)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prepare mirror repo task", mirrorTaskSlogArgs(args.MirrorArgs, task,
			slog.String("before_status", string(beforeStatus)),
			slog.String("error", err.Error()),
		)...)
		return task, err
	}
	slog.InfoContext(ctx, "prepared mirror repo task", mirrorTaskSlogArgs(args.MirrorArgs, task,
		slog.String("before_status", string(beforeStatus)),
		slog.String("after_status", string(task.Status)),
	)...)

	synced, err := w.syncer.SyncRepo(ctx, task.Mirror, task)
	if err != nil {
		if isWorkContextTermination(ctx, err) {
			slog.InfoContext(ctx, "mirror repo sync stopped by execution context",
				mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", err.Error()))...)
			return task, err
		}
		slog.ErrorContext(ctx, "failed to sync mirror repo task", mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", err.Error()))...)
		task.ErrorMessage = err.Error()
		if _, updateErr := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorFail); updateErr != nil {
			slog.ErrorContext(ctx, "failed to update status of mirror repo task", mirrorTaskSlogArgs(args.MirrorArgs, task, slog.String("error", updateErr.Error()))...)
			return task, fmt.Errorf("mark repo sync failed: %w", updateErr)
		}
		return task, err
	}
	if err := ctx.Err(); err != nil {
		return task, err
	}
	syncedTask := synced.Task
	if syncedTask == nil || syncedTask.Mirror == nil || syncedTask.Mirror.Repository == nil {
		return task, fmt.Errorf("synced mirror repo task has no mirror repository")
	}

	// Repositories without LFS objects are already published by the repo syncer;
	// finish the task straight to the terminal LFS-finished state without
	// enqueueing an LFS job.
	if synced.NoLFS {
		syncedTask.Progress = 100
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
			Urgent:       args.Urgent,
		},
		SkipLFSJob: synced.NoLFS,
	}); err != nil {
		return syncedTask, fmt.Errorf("enqueue mirror LFS job: %w", err)
	}
	return syncedTask, nil
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
	worker := newRepoWorker(deps)
	config := newRepoRiverConfigForWorker(deps, worker)
	client, err := workhub.NewWorkClient(ctx, databaseDSN, config)
	if err != nil {
		return nil, err
	}
	manager := client.ConfigureUrgentManager(workhub.UrgentManagerConfig{
		NormalQueue:       workhub.MirrorRepoQueue,
		NormalQueueConfig: config.Queues[workhub.MirrorRepoQueue],
		UrgentIdleDelay:   urgentIdleDelay,
	})
	worker.urgentManager = manager
	return client, nil
}

// newRepoWorker builds the repository worker shared by normal and urgent queues.
func newRepoWorker(deps RepoWorkDeps) *repoWorker {
	return &repoWorker{
		mirrorTaskStore: deps.MirrorTaskStore,
		syncer:          deps.Syncer,
		lfsJobClient:    deps.LFSJobClient,
	}
}

// newRepoRiverConfig builds the River config for repository mirror workers.
func newRepoRiverConfig(deps RepoWorkDeps) *river.Config {
	return newRepoRiverConfigForWorker(deps, newRepoWorker(deps))
}

// newRepoRiverConfigForWorker registers one worker instance for normal and urgent repository queues.
func newRepoRiverConfigForWorker(deps RepoWorkDeps, worker *repoWorker) *river.Config {
	maxWorkers := deps.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	workers := workhub.NewWorkerRegistry(workhub.WorkerOverrides{
		MirrorRepo: worker,
	})

	return &river.Config{
		ErrorHandler: newMirrorJobErrorHandler(deps.MirrorTaskStore),
		Queues: map[string]river.QueueConfig{
			workhub.MirrorRepoQueue:       {MaxWorkers: maxWorkers},
			workhub.MirrorRepoUrgentQueue: {MaxWorkers: workhub.UrgentMaxWorkers(maxWorkers)},
		},
		Workers: workers,
	}
}

// prepareRepoTask moves a queued or retryable repo task into the running state.
func (w *repoWorker) prepareRepoTask(ctx context.Context, task database.MirrorTask) (*database.MirrorTask, error) {
	switch task.Status {
	case types.MirrorRepoSyncStart:
		return &task, nil
	case types.MirrorRepoSyncFailed:
		retried, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorRetry)
		if err != nil {
			return &task, fmt.Errorf("retry mirror repo task: %w", err)
		}
		if retried.Status != types.MirrorQueued {
			return &retried, fmt.Errorf("retried mirror repo task has unexpected status %s", retried.Status)
		}
		task = retried
	case types.MirrorQueued:
	default:
		return &task, fmt.Errorf("cannot prepare mirror repo task with status %s", task.Status)
	}

	started, err := w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
	if err != nil {
		return &task, fmt.Errorf("start mirror repo task: %w", err)
	}
	if started.Status != types.MirrorRepoSyncStart {
		return &started, fmt.Errorf("started mirror repo task has unexpected status %s", started.Status)
	}
	return &started, nil
}

// checkRepoTaskInfo validates repo job ownership and reports tasks whose repo phase no longer needs work.
func checkRepoTaskInfo(task *database.MirrorTask, args workhub.RepoArgs, jobID int64) (bool, error) {
	if err := checkMirrorTaskInfo(task, args.MirrorArgs); err != nil {
		return false, err
	}
	if task.RepoJobID != jobID {
		return false, fmt.Errorf("repo job ID mismatch: task=%d job=%d", task.RepoJobID, jobID)
	}
	switch task.Status {
	case types.MirrorQueued, types.MirrorRepoSyncStart, types.MirrorRepoSyncFailed:
		return false, nil
	default:
		return true, nil
	}
}
