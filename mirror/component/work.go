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
	// terminalStateSnoozeDelay controls retries when an exhausted job cannot persist its terminal task state.
	terminalStateSnoozeDelay = time.Minute
	// workerShutdownSnoozeDelay prevents immediate reacquisition while a worker client stops.
	workerShutdownSnoozeDelay = 5 * time.Second
)

// mirrorWorkConfig supplies the typed differences between repo and LFS lifecycle execution.
type mirrorWorkConfig[A river.JobArgs] struct {
	name             string
	manager          *workhub.UrgentManager
	preemptionDelay  time.Duration
	isUrgent         func(A) bool
	expectedQueue    func(bool) string
	validateQueue    func(A, string) error
	logArgs          func(A, *database.MirrorTask, ...any) []any
	work             func(context.Context, A, int) (*database.MirrorTask, error)
	failureTarget    func(A) mirrorJobFailureTarget
	failureFinalizer *mirrorJobFailureFinalizer
}

// mirrorTaskStore provides task state operations shared by repo, LFS, and final error handling.
type mirrorTaskStore interface {
	FindByID(ctx context.Context, ID int64) (*database.MirrorTask, error)
	UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, statusAction string) (database.MirrorTask, error)
}

// runMirrorWork applies the shared River lifecycle around typed mirror business work.
func runMirrorWork[A river.JobArgs](ctx context.Context, job *river.Job[A], config mirrorWorkConfig[A]) (workErr error) {
	riverCtx := ctx
	args := job.Args
	urgent := config.isUrgent(args)
	var task *database.MirrorTask
	exitAttrs := []any{
		slog.Int64("job_id", job.ID),
		slog.Bool("urgent", urgent),
		slog.String("expected_queue", config.expectedQueue(urgent)),
		slog.String("actual_queue", job.Queue),
	}
	defer func() {
		var (
			snooze     = false
			panicValue = recover()
			level      = slog.LevelInfo
			success    = workErr == nil && panicValue == nil
		)
		if panicValue != nil {
			level = slog.LevelError
			exitAttrs = append(exitAttrs, slog.Any("panic", panicValue))
			if job.Attempt >= job.MaxAttempts && config.failureFinalizer != nil && config.failureTarget != nil {
				panicMessage := fmt.Sprintf("worker panic: %v", panicValue)
				if err := config.failureFinalizer.finalize(
					riverCtx, job.ID, job.Kind, job.Attempt, config.failureTarget(args), panicMessage,
					slog.Any("panic", panicValue),
				); err != nil {
					workErr = river.JobSnooze(terminalStateSnoozeDelay)
					snooze = true
					exitAttrs = append(exitAttrs,
						slog.String("reason", "terminal_state_persistence"),
						slog.String("finalizer_error", err.Error()),
						slog.Duration("snooze_delay", terminalStateSnoozeDelay),
					)
					panicValue = nil
				}
			}
		} else if workErr != nil {
			originalErr := workErr
			var (
				cancelErr *river.JobCancelError
				snoozeErr *river.JobSnoozeError
			)
			snooze = true

			switch {
			case isUrgentPreemption(riverCtx, ctx, originalErr):
				workErr = river.JobSnooze(config.preemptionDelay)
				exitAttrs = append(exitAttrs,
					slog.String("reason", "urgent_preemption"),
					slog.String("state", string(config.manager.State())),
					slog.Duration("snooze_delay", config.preemptionDelay),
				)
			case errors.Is(riverCtx.Err(), context.DeadlineExceeded) &&
				errors.Is(originalErr, context.DeadlineExceeded):
				workErr = river.JobSnooze(0)
				exitAttrs = append(exitAttrs,
					slog.String("reason", "worker_timeout"),
					slog.Duration("snooze_delay", 0),
				)
			case errors.Is(riverCtx.Err(), context.Canceled) && errors.Is(originalErr, context.Canceled):
				workErr = river.JobSnooze(workerShutdownSnoozeDelay)
				exitAttrs = append(exitAttrs,
					slog.String("reason", "worker_shutdown"),
					slog.Duration("snooze_delay", workerShutdownSnoozeDelay),
				)
			case errors.As(originalErr, &snoozeErr):
				exitAttrs = append(exitAttrs, slog.Duration("snooze_delay", snoozeErr.Duration))
				if config.manager != nil && config.manager.State() == workhub.UrgentStateClosed {
					exitAttrs = append(exitAttrs,
						slog.String("reason", "worker_shutdown"),
						slog.String("state", string(workhub.UrgentStateClosed)),
					)
				}
			case errors.As(originalErr, &cancelErr):
				snooze = false
				level = slog.LevelError
			default:
				snooze = false
				level = slog.LevelError
				if job.Attempt >= job.MaxAttempts && config.failureFinalizer != nil && config.failureTarget != nil {
					if err := config.failureFinalizer.finalize(
						riverCtx, job.ID, job.Kind, job.Attempt, config.failureTarget(args), originalErr.Error(),
						slog.String("error", originalErr.Error()),
					); err != nil {
						workErr = river.JobSnooze(terminalStateSnoozeDelay)
						snooze = true
						exitAttrs = append(exitAttrs,
							slog.String("reason", "terminal_state_persistence"),
							slog.String("finalizer_error", err.Error()),
							slog.Duration("snooze_delay", terminalStateSnoozeDelay),
						)
					}
				}
			}
			exitAttrs = append(exitAttrs,
				slog.String("error", originalErr.Error()),
				slog.Any("context", riverCtx.Err()),
			)
		}
		exitAttrs = append(exitAttrs,
			slog.Bool("success", success),
			slog.Bool("snooze", snooze),
		)
		slog.Log(riverCtx, level, config.name+" job work exited", config.logArgs(args, task, exitAttrs...)...)
		if panicValue != nil {
			panic(panicValue)
		}
	}()
	slog.InfoContext(ctx, "working on "+config.name+" job", config.logArgs(args, task,
		slog.Int("attempts", job.Attempt),
		slog.Int("max_attempts", job.MaxAttempts),
		slog.Int("priority", job.Priority),
		slog.Int64("job_id", job.ID),
	)...)
	if job.JobRow != nil && job.Queue != "" {
		if err := config.validateQueue(args, job.Queue); err != nil {
			slog.ErrorContext(ctx, "canceling "+config.name+" job with queue mismatch", config.logArgs(args, task,
				slog.Int64("job_id", job.ID),
				slog.String("expected_queue", config.expectedQueue(urgent)),
				slog.String("actual_queue", job.Queue),
				slog.String("error", err.Error()),
			)...)
			return river.JobCancel(err)
		}
	}

	var done func()
	if config.manager != nil && !urgent {
		normalCtx, normalDone, allowed := config.manager.BeginNormal(riverCtx)
		if !allowed {
			state := config.manager.State()
			if state == workhub.UrgentStateClosed {
				workErr = river.JobSnooze(workerShutdownSnoozeDelay)
				return workErr
			}
			exitAttrs = append(exitAttrs,
				slog.String("reason", "urgent_work_blocks_execution"),
				slog.String("state", string(state)),
			)
			workErr = river.JobSnooze(config.preemptionDelay)
			return workErr
		}
		ctx, done = normalCtx, normalDone
		defer done()
	}
	task, workErr = config.work(ctx, args, mirrorJobRetryCount(job.Attempt))
	return workErr
}

// mirrorJobRetryCount converts River's one-based attempt number to completed retry count.
func mirrorJobRetryCount(attempt int) int {
	if attempt <= 1 {
		return 0
	}
	return attempt - 1
}

// beginUrgentWork reserves urgent execution and converts worker shutdown into a short River snooze.
func beginUrgentWork(manager *workhub.UrgentManager, riverCtx context.Context) (func(), error) {
	done, err := manager.BeginUrgent(riverCtx)
	if errors.Is(err, workhub.ErrWorkerShutdown) {
		return nil, river.JobSnooze(workerShutdownSnoozeDelay)
	}
	return done, err
}

// contextCauseError returns the standard context cancellation or deadline error.
func contextCauseError(ctx context.Context) error {
	return ctx.Err()
}

// isUrgentPreemption identifies local urgent cancellation without treating River cancellation as preemption.
func isUrgentPreemption(riverCtx, ctx context.Context, workErr error) bool {
	return riverCtx.Err() == nil && isUrgentWorkCancellation(ctx, workErr)
}

// isUrgentWorkCancellation reports whether urgent preemption stopped business work.
func isUrgentWorkCancellation(ctx context.Context, workErr error) bool {
	return errors.Is(context.Cause(ctx), workhub.ErrUrgentPreempt) &&
		(errors.Is(workErr, context.Canceled) ||
			errors.Is(workErr, context.DeadlineExceeded) ||
			errors.Is(workErr, workhub.ErrUrgentPreempt))
}
