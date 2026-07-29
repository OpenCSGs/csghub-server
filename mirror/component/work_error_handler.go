package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// mirrorJobFailureFinalizer advances exhausted mirror jobs to their business terminal state.
type mirrorJobFailureFinalizer struct {
	taskStore mirrorTaskStore
}

// mirrorJobErrorHandler finalizes errors and panics observed by River outside worker return handling.
type mirrorJobErrorHandler struct {
	finalizer *mirrorJobFailureFinalizer
}

// mirrorJobFinalizationError preserves the trusted persisted status for structured logging.
type mirrorJobFinalizationError struct {
	taskStatus types.MirrorTaskStatus
	action     string
	err        error
}

// Error describes the failed task transition.
func (e *mirrorJobFinalizationError) Error() string {
	return fmt.Sprintf("update mirror task from %s with action %s: %v", e.taskStatus, e.action, e.err)
}

// Unwrap exposes the underlying persistence error.
func (e *mirrorJobFinalizationError) Unwrap() error {
	return e.err
}

// mirrorJobFailureTarget identifies the task stage associated with a River job.
type mirrorJobFailureTarget struct {
	taskID         int64
	stage          string
	fatalStatus    types.MirrorTaskStatus
	persistedJobID func(*database.MirrorTask) int64
}

// newMirrorJobFailureFinalizer creates the shared exhausted-job finalizer.
func newMirrorJobFailureFinalizer(taskStore mirrorTaskStore) *mirrorJobFailureFinalizer {
	return &mirrorJobFailureFinalizer{taskStore: taskStore}
}

// newMirrorJobErrorHandler creates the final retry handler shared by repo and LFS workers.
func newMirrorJobErrorHandler(taskStore mirrorTaskStore) *mirrorJobErrorHandler {
	return &mirrorJobErrorHandler{finalizer: newMirrorJobFailureFinalizer(taskStore)}
}

// HandleError marks an active mirror stage fatal when the current attempt is the last one.
func (h *mirrorJobErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	if errors.Is(err, context.Canceled) && errors.Is(context.Cause(ctx), context.Canceled) {
		return nil
	}
	h.finalizeAfterRetries(ctx, job, err.Error(), slog.String("error", err.Error()))
	return nil
}

// HandlePanic applies the same final-attempt handling when a River worker panics.
func (h *mirrorJobErrorHandler) HandlePanic(
	ctx context.Context,
	job *rivertype.JobRow,
	panicValue any,
	trace string,
) *river.ErrorHandlerResult {
	h.finalizeAfterRetries(ctx, job, fmt.Sprintf("worker panic: %v", panicValue),
		slog.Any("panic", panicValue),
		slog.String("panic_trace", trace),
	)
	return nil
}

// finalizeAfterRetries applies the shared finalizer and records failures that River cannot retry from an error handler.
func (h *mirrorJobErrorHandler) finalizeAfterRetries(
	ctx context.Context,
	job *rivertype.JobRow,
	errorMessage string,
	causeAttrs ...any,
) {
	if job == nil || job.Attempt < job.MaxAttempts {
		return
	}
	target, err := mirrorJobTarget(job)
	if err != nil {
		slog.ErrorContext(ctx, "failed to identify exhausted mirror job",
			slog.Int64("job_id", job.ID),
			slog.String("job_kind", job.Kind),
			slog.String("error", err.Error()),
		)
		return
	}
	if err := h.finalizer.finalize(ctx, job.ID, job.Kind, job.Attempt, target, errorMessage, causeAttrs...); err != nil {
		attrs := mirrorJobFailureAttrs(job.ID, job.Kind, job.Attempt, job.MaxAttempts, target, causeAttrs...)
		var finalizationErr *mirrorJobFinalizationError
		if errors.As(err, &finalizationErr) {
			attrs = append(attrs,
				slog.String("task_status", string(finalizationErr.taskStatus)),
				slog.String("action", finalizationErr.action),
			)
		}
		slog.ErrorContext(ctx, "failed to mark mirror task fatal after retries exhausted",
			append(attrs, slog.String("handler_error", err.Error()))...,
		)
	}
}

// finalize advances the latest persisted task through valid FSM transitions to its fatal state.
func (f *mirrorJobFailureFinalizer) finalize(
	ctx context.Context,
	jobID int64,
	jobKind string,
	attempt int,
	target mirrorJobFailureTarget,
	errorMessage string,
	causeAttrs ...any,
) error {
	attrs := mirrorJobFailureAttrs(jobID, jobKind, 0, 0, target, causeAttrs...)
	task, err := f.taskStore.FindByID(ctx, target.taskID)
	if err != nil {
		return fmt.Errorf("load mirror task %d: %w", target.taskID, err)
	}
	if task == nil {
		slog.WarnContext(ctx, "skip exhausted mirror job for missing task", attrs...)
		return nil
	}
	if target.persistedJobID(task) != jobID {
		slog.WarnContext(ctx, "skip exhausted stale mirror job",
			append(attrs, slog.Int64("current_job_id", target.persistedJobID(task)))...,
		)
		return nil
	}
	if task.Status == target.fatalStatus {
		return nil
	}
	actions := mirrorJobFatalActions(jobKind, task.Status)
	if len(actions) == 0 {
		slog.WarnContext(ctx, "skip fatal transition for mirror task in unexpected status",
			append(attrs, slog.String("task_status", string(task.Status)))...,
		)
		return nil
	}

	updatedTask := *task
	updatedTask.RetryCount = mirrorJobRetryCount(attempt)
	initialStatus := task.Status
	for _, action := range actions {
		updatedTask.ErrorMessage = errorMessage
		nextTask, updateErr := f.taskStore.UpdateStatusAndRepoSyncStatus(ctx, updatedTask, action)
		if updateErr != nil {
			if f.finalizedConcurrently(ctx, jobID, jobKind, target) {
				return nil
			}
			return &mirrorJobFinalizationError{
				taskStatus: updatedTask.Status,
				action:     action,
				err:        updateErr,
			}
		}
		updatedTask = nextTask
	}
	slog.ErrorContext(ctx, "mirror job retries exhausted; marked task fatal",
		append(attrs,
			slog.String("initial_task_status", string(initialStatus)),
			slog.String("final_task_status", string(updatedTask.Status)),
		)...,
	)
	return nil
}

// finalizedConcurrently reports whether another transaction made finalization unnecessary.
func (f *mirrorJobFailureFinalizer) finalizedConcurrently(
	ctx context.Context,
	jobID int64,
	jobKind string,
	target mirrorJobFailureTarget,
) bool {
	task, err := f.taskStore.FindByID(ctx, target.taskID)
	if err != nil || task == nil {
		return false
	}
	if target.persistedJobID(task) != jobID || task.Status == target.fatalStatus {
		return true
	}
	return len(mirrorJobFatalActions(jobKind, task.Status)) == 0
}

// mirrorJobFailureAttrs creates common structured fields for finalization logs.
func mirrorJobFailureAttrs(
	jobID int64,
	jobKind string,
	attempt int,
	maxAttempts int,
	target mirrorJobFailureTarget,
	causeAttrs ...any,
) []any {
	attrs := []any{
		slog.Int64("job_id", jobID),
		slog.String("job_kind", jobKind),
		slog.Int64("mirror_task_id", target.taskID),
		slog.String("stage", target.stage),
	}
	if attempt > 0 || maxAttempts > 0 {
		attrs = append(attrs,
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", maxAttempts),
		)
	}
	return append(attrs, causeAttrs...)
}

// mirrorJobFatalActions returns the valid FSM path from an active stage status to fatal.
func mirrorJobFatalActions(jobKind string, status types.MirrorTaskStatus) []string {
	switch jobKind {
	case workhub.MirrorRepoQueue:
		switch status {
		case types.MirrorQueued:
			return []string{database.MirrorContinue, database.MirrorFail, database.MirrorFatal}
		case types.MirrorRepoSyncStart:
			return []string{database.MirrorFail, database.MirrorFatal}
		case types.MirrorRepoSyncFailed:
			return []string{database.MirrorFatal}
		}
	case workhub.MirrorLFSQueue:
		switch status {
		case types.MirrorRepoSyncFinished:
			return []string{database.MirrorContinue, database.MirrorFail, database.MirrorFatal}
		case types.MirrorLfsSyncStart:
			return []string{database.MirrorFail, database.MirrorFatal}
		case types.MirrorLfsSyncFailed:
			return []string{database.MirrorFatal}
		}
	}
	return nil
}

// mirrorRepoJobFailureTarget builds the finalization target for repository arguments.
func mirrorRepoJobFailureTarget(args workhub.RepoArgs) mirrorJobFailureTarget {
	return mirrorJobFailureTarget{
		taskID:      args.MirrorTaskID,
		stage:       "repo",
		fatalStatus: types.MirrorRepoSyncFatal,
		persistedJobID: func(task *database.MirrorTask) int64 {
			return task.RepoJobID
		},
	}
}

// mirrorLFSJobFailureTarget builds the finalization target for LFS arguments.
func mirrorLFSJobFailureTarget(args workhub.LFSArgs) mirrorJobFailureTarget {
	return mirrorJobFailureTarget{
		taskID:      args.MirrorTaskID,
		stage:       "lfs",
		fatalStatus: types.MirrorLfsSyncFatal,
		persistedJobID: func(task *database.MirrorTask) int64 {
			return task.LFSJobID
		},
	}
}

// mirrorJobTarget decodes stable task identifiers from supported mirror job kinds.
func mirrorJobTarget(job *rivertype.JobRow) (mirrorJobFailureTarget, error) {
	switch job.Kind {
	case workhub.MirrorRepoQueue:
		var args workhub.RepoArgs
		if err := json.Unmarshal(job.EncodedArgs, &args); err != nil {
			return mirrorJobFailureTarget{}, fmt.Errorf("decode repo job args: %w", err)
		}
		return mirrorRepoJobFailureTarget(args), nil
	case workhub.MirrorLFSQueue:
		var args workhub.LFSArgs
		if err := json.Unmarshal(job.EncodedArgs, &args); err != nil {
			return mirrorJobFailureTarget{}, fmt.Errorf("decode LFS job args: %w", err)
		}
		return mirrorLFSJobFailureTarget(args), nil
	default:
		return mirrorJobFailureTarget{}, fmt.Errorf("unsupported mirror job kind %q", job.Kind)
	}
}
