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

// mirrorJobErrorHandler logs errors and panics observed by River outside worker return handling.
type mirrorJobErrorHandler struct {
	taskStore mirrorTaskStore
}

// newMirrorJobErrorHandler creates the shared mirror job error logger.
func newMirrorJobErrorHandler(taskStore mirrorTaskStore) *mirrorJobErrorHandler {
	return &mirrorJobErrorHandler{taskStore: taskStore}
}

// HandleError logs a River job error unless the job was cancelled remotely.
func (h *mirrorJobErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	if (errors.Is(err, river.ErrJobCancelledRemotely) || errors.Is(context.Cause(ctx), river.ErrJobCancelledRemotely)) ||
		(errors.Is(err, context.Canceled) && errors.Is(context.Cause(ctx), context.Canceled)) {
		return nil
	}
	if err == nil {
		return nil
	}

	slog.ErrorContext(ctx, "mirror job error",
		slog.String("error", err.Error()),
		slog.Int64("job_id", job.ID),
		slog.String("job_kind", job.Kind),
		slog.Int("attempt", job.Attempt),
		slog.Int("max_attempts", job.MaxAttempts),
		slog.Any("handle_error", workErrorHandle(ctx, h.taskStore, job, err.Error())),
	)
	return nil
}

// HandlePanic logs a panic and its River job context.
func (h *mirrorJobErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicValue any, trace string) *river.ErrorHandlerResult {
	if panicValue == nil {
		return nil
	}
	slog.ErrorContext(ctx, "mirror job panic",
		slog.Any("job_args", string(job.EncodedArgs)),
		slog.Any("panic", panicValue),
		slog.String("panic_trace", trace),
		slog.Int64("job_id", job.ID),
		slog.String("job_kind", job.Kind),
		slog.Int("attempt", job.Attempt),
		slog.Int("max_attempts", job.MaxAttempts),
		slog.Any("handle_error", workErrorHandle(ctx, h.taskStore, job, "work panic")),
	)
	return nil
}

func workErrorHandle(ctx context.Context, taskStore mirrorTaskStore, job *rivertype.JobRow, errMsg string) error {
	if job.Attempt < job.MaxAttempts {
		return nil
	}

	var args workhub.MirrorArgs
	if err := json.Unmarshal(job.EncodedArgs, &args); err != nil {
		return fmt.Errorf("unmarshal mirror job error: %w", err)
	}

	slog.ErrorContext(ctx, "mirror max attempt, set job fatal status",
		slog.Int64("job_id", job.ID),
		slog.Int64("mirror_id", args.MirrorID),
		slog.Int64("task_id", args.MirrorTaskID),
		slog.Int64("repo_id", args.RepositoryID),
		slog.Bool("urgent", args.Urgent),
	)

	task, err := taskStore.FindByID(ctx, args.MirrorTaskID)
	if err != nil {
		return fmt.Errorf("find mirror job error: %w", err)
	}

	slog.ErrorContext(ctx, "mirror job fatal status",
		slog.Int64("task_id", args.MirrorTaskID),
		slog.String("status", string(task.Status)),
	)

	if task.Status == types.MirrorRepoSyncFatal || task.Status == types.MirrorLfsSyncFatal {
		return nil
	}

	switch job.Kind {
	case workhub.RepoArgs{}.Kind():
		switch task.Status {
		case types.MirrorQueued, types.MirrorRepoSyncStart, types.MirrorRepoSyncFailed:
			if task.RepoJobID != job.ID {
				return fmt.Errorf("repo job id mismatch, task.RepoJobID = %d, job.ID=%d", task.RepoJobID, job.ID)
			}
		default:
			return nil
		}
	case workhub.LFSArgs{}.Kind():
		switch task.Status {
		case types.MirrorRepoSyncFinished, types.MirrorLfsSyncStart, types.MirrorLfsSyncFailed:
			if task.LFSJobID != job.ID {
				return fmt.Errorf("lfs job id mismatch, task.LFSJobID = %d, job.ID=%d", task.LFSJobID, job.ID)
			}
		default:
			return nil
		}
	default:
		return fmt.Errorf("unknown job kind: %s", job.Kind)
	}

	if task.ErrorMessage == "" && errMsg != "" {
		task.ErrorMessage = errMsg
	}
	task.RetryCount = job.Attempt
	if _, err = taskStore.UpdateStatusAndRepoSyncStatus(ctx, *task, database.MirrorFatal); err != nil {
		return fmt.Errorf("update mirror job error: %w", err)
	}
	return nil
}
