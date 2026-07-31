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
)

const (
	// urgentJobDelay controls how long preempted normal jobs wait before retrying.
	urgentJobDelay = time.Minute
	// urgentIdleDelay is the quiet period before normal queue restoration.
	urgentIdleDelay = 10 * time.Second
	// workerShutdownSnoozeDelay prevents immediate reacquisition while a worker client stops.
	workerShutdownSnoozeDelay = 30 * time.Second
)

// mirrorWorkConfig contains immutable job metadata, lifecycle dependencies, and typed business work.
type mirrorWorkConfig[T river.JobArgs] struct {
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

func runWorkWithLog[T river.JobArgs](ctx context.Context, job *river.Job[T], cfg mirrorWorkConfig[T]) error {
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

func runWork[T river.JobArgs](ctx context.Context, job *river.Job[T], cfg mirrorWorkConfig[T]) error {
	workCtx := ctx

	if !cfg.urgent {
		// Register normal work with the urgent-work manager.
		normalCtx, normalDone, allowed := cfg.manager.BeginNormal(ctx)
		if !allowed {
			// Snooze rejected work and retry the pause transition in case an earlier update failed.
			_ = pauseTaskWork(workCtx, cfg)
			return river.JobSnooze(urgentJobDelay)
		}
		defer normalDone()

		// Pass the manager-derived context to the business work.
		ctx = normalCtx
	} else {
		// Reserve urgent execution and preempt normal work.
		urgentDone, err := beginUrgentWork(cfg.manager, ctx)
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

	var (
		cancelErr *river.JobCancelError
		snoozeErr *river.JobSnoozeError
	)
	switch {
	// Resume locally preempted work after the urgent-job delay.
	case isUrgentPreemption(workCtx, ctx, workErr):
		workErr = river.JobSnooze(urgentJobDelay)
		if err := pauseTaskWork(workCtx, cfg); err != nil {
			workErr = errors.Join(workErr, fmt.Errorf("pause preempted mirror task: %w", err))
		}
	// Retry timed-out work immediately without changing its business status.
	case errors.Is(workCtx.Err(), context.DeadlineExceeded) && errors.Is(workErr, context.DeadlineExceeded):
		workErr = river.JobSnooze(0)
	// Worker shutdown and Job.Cancel both cancel the River context.
	// The Job.Cancel caller owns the task status update, so this path leaves it unchanged.
	case errors.Is(workCtx.Err(), context.Canceled) && errors.Is(workErr, context.Canceled):
		return river.JobSnooze(workerShutdownSnoozeDelay)
	// Honor a JobSnooze returned explicitly by business work.
	case errors.As(workErr, &snoozeErr):
		if snoozeErr.Duration > time.Second {
			// Attempt to pause the task; the upcoming retry can recover from an update failure.
			_ = pauseTaskWork(workCtx, cfg)
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

// beginUrgentWork reserves urgent execution and converts worker shutdown into a short River snooze.
func beginUrgentWork(manager *workhub.UrgentManager, riverCtx context.Context) (func(), error) {
	done, err := manager.BeginUrgent(riverCtx)
	if errors.Is(err, workhub.ErrWorkerShutdown) {
		return nil, river.JobSnooze(workerShutdownSnoozeDelay)
	}
	return done, err
}

func pauseTaskWork[T river.JobArgs](ctx context.Context, cfg mirrorWorkConfig[T]) error {
	task, err := cfg.taskStore.FindByID(ctx, cfg.args.MirrorTaskID)
	if err != nil {
		return fmt.Errorf("find mirror job error: %w", err)
	}
	_, err = cfg.taskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorPause)
	return err
}

// isUrgentPreemption identifies local urgent cancellation without treating River cancellation as preemption.
func isUrgentPreemption(riverCtx, ctx context.Context, workErr error) bool {
	return riverCtx.Err() == nil && isUrgentWorkCancellation(ctx, workErr)
}

// isUrgentWorkCancellation reports whether urgent preemption stopped business work.
func isUrgentWorkCancellation(ctx context.Context, err error) bool {
	return errors.Is(context.Cause(ctx), workhub.ErrUrgentPreempt) &&
		(errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, workhub.ErrUrgentPreempt))
}

// isWorkContextTermination reports whether work stopped because its execution context ended.
func isWorkContextTermination(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}
	return isUrgentWorkCancellation(ctx, err) ||
		errors.Is(err, ctx.Err()) ||
		errors.Is(err, context.Cause(ctx))
}
