package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/riverqueue/river"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
	sharedcomponent "opencsg.com/csghub-server/component"
)

const (
	// urgentIdleDelay is the quiet period before normal queue restoration.
	urgentIdleDelay = 10 * time.Second
	// mirrorPauseCleanupTimeout bounds business-state cleanup after execution cancellation.
	mirrorPauseCleanupTimeout = 10 * time.Second
)

// newMirrorJobLeaseConfig reserves one cancellation budget for compensation,
// business cleanup and River completion. FinalizeTimeout cannot extend an
// earlier cancellation deadline: allow 30s for repo compensation, 10s for
// pause cleanup, 30s for completion and 20s of scheduling/transport headroom.
func newMirrorJobLeaseConfig() *river.JobLeaseConfig {
	return &river.JobLeaseConfig{
		CancelGracePeriod: 90 * time.Second,
		LocalSafetyMargin: 100 * time.Second,
		LeaseDuration:     180 * time.Second,
		FinalizeTimeout:   30 * time.Second,
	}
}

// mirrorWorkConfig contains immutable job metadata, lifecycle dependencies, and typed business work.
type mirrorWorkConfig[T river.JobArgs] struct {
	reporter  *sharedcomponent.MirrorMetricsReporter
	args      workhub.MirrorArgs
	urgent    bool
	stage     string // repo/lfs
	manager   *workhub.UrgentManager
	taskStore mirrorTaskStore
	work      func(context.Context, T, int) (*database.MirrorTask, error)
}

// mirrorTaskStore provides task state operations shared by repo, LFS, and final error handling.
type mirrorTaskStore interface {
	FindByID(ctx context.Context, ID int64) (*database.MirrorTask, error)
	UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, statusAction string) (database.MirrorTask, error)
}

// runWorkWithLog records lifecycle outcomes without changing the business result.
func runWorkWithLog[T river.JobArgs](ctx context.Context, job *river.Job[T], cfg mirrorWorkConfig[T]) error {
	ctx, release := cfg.reporter.Track(ctx, metricJobDetails(cfg.args, cfg.stage))
	defer release()
	workLogs := []any{
		slog.Int64("job_id", job.ID),
		slog.String("stage", cfg.stage),
		slog.Int("attempt", job.Attempt),
		slog.Int("max_attempt", job.MaxAttempts),
		slog.Int("priority", job.Priority),
		slog.String("kind", job.Kind),
		slog.String("queue", job.Queue),
		slog.Bool("urgent", cfg.urgent),
		slog.Int64("mirror_id", cfg.args.MirrorID),
		slog.Int64("repo_id", cfg.args.RepositoryID),
		slog.Int64("mirror_task_id", cfg.args.MirrorTaskID),
	}
	slog.InfoContext(ctx, fmt.Sprintf("[%s] work start", cfg.stage), workLogs...)

	var execError error
	defer func() {
		panicValue := recover()
		slog.InfoContext(ctx, fmt.Sprintf("[%s] work exit", cfg.stage),
			append(workLogs,
				slog.Bool("success", execError == nil && panicValue == nil),
				slog.Any("error", execError),
			)...,
		)
		if panicValue != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("[%s] work panic", cfg.stage),
				append(workLogs, slog.Any("panic", panicValue))...,
			)

			_ = workErrorHandle(ctx, cfg.taskStore, job.JobRow, "work panic")
			panic(panicValue)
		}
	}()

	execError = runWork(ctx, job, cfg)
	return execError
}

// runWork gates urgent execution and classifies business and lifecycle errors.
func runWork[T river.JobArgs](ctx context.Context, job *river.Job[T], cfg mirrorWorkConfig[T]) error {
	if cfg.urgent {
		// Reserve urgent execution and preempt normal work.
		urgentDone, err := cfg.manager.BeginUrgent(ctx)
		if err != nil {
			// Reservation usually fails during worker shutdown or context cancellation.
			return err
		}

		// Release the urgent reservation when the work finishes.
		defer urgentDone()
	}

	_, workErr := cfg.work(ctx, job.Args, job.Attempt)
	if workErr == nil {
		return nil
	}

	var cancelErr *river.JobCancelError
	switch {
	// Normalize only pure pause cancellation after business cleanup so River preserves claim order.
	case isQueuePauseCancellation(ctx, workErr):
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mirrorPauseCleanupTimeout)
		defer cancel()
		if err := pauseTaskWork(cleanupCtx, cfg, job.ID); err != nil {
			return errors.Join(workErr, fmt.Errorf("pause interrupted mirror task: %w", err))
		}
		slog.InfoContext(ctx, "mirror work paused", slog.Int64("job_id", job.ID), slog.String("stage", cfg.stage), slog.Any("error", workErr))
		// River recognizes its own pause error, including when the original cancellation came from gRPC.
		workErr = river.ErrQueuePaused
	// Worker shutdown and Job.Cancel both cancel the River context.
	// The Job.Cancel caller owns the task status update, so this path leaves it unchanged.
	case isWorkCancellation(ctx, workErr):
		if !errors.Is(workErr, context.Canceled) {
			slog.InfoContext(ctx, "mirror work canceled", slog.Int64("job_id", job.ID), slog.String("stage", cfg.stage), slog.Any("error", workErr))
			// River recognizes Go cancellation when settling client shutdown.
			workErr = context.Canceled
		}
	// JobCancel stops retries for invalid job metadata and leaves the task status unchanged.
	// Business errors that cannot be retried must use the task state machine instead.
	case errors.As(workErr, &cancelErr):
	// Treat every remaining error as a business-work failure.
	default:
		// Promote the task to fatal when the business error exhausts all attempts.
		workErr = errors.Join(workErr, workErrorHandle(ctx, cfg.taskStore, job.JobRow, workErr.Error()))
	}
	return workErr
}

// mirrorTaskSlogArgs appends job identity and available task details to shared lifecycle logs.
func mirrorTaskSlogArgs(args workhub.MirrorArgs, task *database.MirrorTask, attrs ...any) []any {
	attrs = append(attrs,
		slog.Int64("mirror_id", args.MirrorID),
		slog.Int64("repository_id", args.RepositoryID),
		slog.Int64("mirror_task_id", args.MirrorTaskID),
		slog.Bool("urgent", args.Urgent),
	)
	if task == nil {
		return append(attrs, slog.Any("task_status", nil))
	}
	attrs = append(attrs, slog.String("task_status", string(task.Status)))
	if task.Mirror == nil {
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

// checkMirrorTaskInfo validates the task identity shared by repo and LFS jobs.
func checkMirrorTaskInfo(task *database.MirrorTask, args workhub.MirrorArgs) error {
	if task == nil {
		return fmt.Errorf("mirror task is nil")
	}
	if task.Mirror == nil {
		return fmt.Errorf("mirror task %d has no mirror", task.ID)
	}
	if args.MirrorID != 0 && task.MirrorID != args.MirrorID {
		return fmt.Errorf("mirror ID mismatch: task=%d job=%d", task.MirrorID, args.MirrorID)
	}
	if args.RepositoryID != 0 && task.Mirror.RepositoryID != args.RepositoryID {
		return fmt.Errorf("repository ID mismatch: task=%d job=%d", task.Mirror.RepositoryID, args.RepositoryID)
	}
	if task.Mirror.CurrentTaskID != 0 && task.Mirror.CurrentTaskID != task.ID {
		return fmt.Errorf("mirror current task mismatch: current=%d job_task=%d", task.Mirror.CurrentTaskID, task.ID)
	}
	return nil
}

// pauseTaskWork returns the current stage to waiting without touching River storage.
func pauseTaskWork[T river.JobArgs](ctx context.Context, cfg mirrorWorkConfig[T], jobID int64) error {
	task, err := cfg.taskStore.FindByID(ctx, cfg.args.MirrorTaskID)
	if err != nil {
		return fmt.Errorf("find mirror job error: %w", err)
	}
	if err := checkMirrorTaskInfo(task, cfg.args); err != nil {
		return err
	}
	switch cfg.stage {
	case "repo":
		if task.RepoJobID != jobID {
			return fmt.Errorf("repo job ID mismatch: task=%d job=%d", task.RepoJobID, jobID)
		}
	case "lfs":
		if task.LFSJobID != jobID {
			return fmt.Errorf("lfs job ID mismatch: task=%d job=%d", task.LFSJobID, jobID)
		}
	default:
		return fmt.Errorf("unknown mirror work stage: %s", cfg.stage)
	}
	_, err = cfg.taskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorPause)
	return err
}

// isWorkCancellation excludes queue pauses, explicit cancellation and business errors.
func isWorkCancellation(ctx context.Context, err error) bool {
	if ctx.Err() != context.Canceled || errors.Is(context.Cause(ctx), river.ErrQueuePaused) {
		return false
	}
	var cancelErr *river.JobCancelError
	return !errors.As(err, &cancelErr) && isCancellationOnly(err)
}

// isCancellationOnly requires every error branch to represent execution cancellation.
func isCancellationOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !isCancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return isCancellationOnly(wrapped)
	}
	// Inspect RPC status only at a leaf to preserve joined business errors.
	return errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled
}

// isQueuePauseCancellation recognizes pure local pause errors, including RPC cancellations normalized before returning to River.
func isQueuePauseCancellation(ctx context.Context, err error) bool {
	var cancelErr *river.JobCancelError
	return errors.Is(context.Cause(ctx), river.ErrQueuePaused) &&
		!errors.As(err, &cancelErr) && isPauseCancellationOnly(err)
}

// isPauseCancellationOnly checks every joined error so checkpoint failures retain failure semantics.
func isPauseCancellationOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !isPauseCancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return isPauseCancellationOnly(wrapped)
	}
	// Inspect RPC status only at a leaf; checking a joined error can overlook its business-error branches.
	return errors.Is(err, context.Canceled) || errors.Is(err, river.ErrQueuePaused) || status.Code(err) == codes.Canceled
}

// isWorkContextTermination reports whether work stopped because its execution context ended.
func isWorkContextTermination(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}
	if errors.Is(context.Cause(ctx), river.ErrQueuePaused) {
		return isQueuePauseCancellation(ctx, err)
	}
	return errors.Is(err, ctx.Err()) ||
		errors.Is(err, context.Cause(ctx))
}

// metricJobDetails copies only the identifiers needed to display current work.
func metricJobDetails(args workhub.MirrorArgs, stage string) types.MirrorMetricsTask {
	return types.MirrorMetricsTask{TaskID: strconv.FormatInt(args.MirrorTaskID, 10), Stage: stage, Urgent: args.Urgent}
}

// metricTaskDetails copies the validated repository path and business start time.
func metricTaskDetails(task *database.MirrorTask) types.MirrorMetricsTask {
	result := types.MirrorMetricsTask{}
	if !task.StartedAt.IsZero() {
		at := task.StartedAt.UnixMilli()
		result.StartedAtMS = &at
	}
	if task.Mirror != nil && task.Mirror.Repository != nil {
		repo := task.Mirror.Repository
		result.RepoPath = string(repo.RepositoryType) + "s/" + repo.Path
	}
	return result
}
