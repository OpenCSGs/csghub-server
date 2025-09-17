package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	v1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

type ImageBuilderExecutor interface {
}

type imagebuilderExecutorImpl struct {
	cfg   *config.Config
	store database.DeployTaskStore
}

var _ WebHookExecutor = (*imagebuilderExecutorImpl)(nil)

func NewImageBuilderExecutor(config *config.Config) (ImageBuilderExecutor, error) {
	executor := &imagebuilderExecutorImpl{
		cfg:   config,
		store: database.NewDeployTaskStore(),
	}
	// register the heartbeat executor for webhook callback func ProcessEvent
	err := RegisterWebHookExecutor(types.RunnerBuilderCreate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register heartbeat executor: %w", err)
	}

	err = RegisterWebHookExecutor(types.RunnerBuilderChange, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register heartbeat executor: %w", err)
	}
	return executor, nil

}

func (h *imagebuilderExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	slog.Debug("heartbeat event invoked", slog.Any("event", event))
	var data types.ImageBuilderEvent
	err := json.Unmarshal(event.Data, &data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	switch event.EventType {
	case types.RunnerBuilderChange:
		task, err := h.store.GetDeployTask(ctx, data.TaskId)
		if err != nil {
			return fmt.Errorf("failed to get deploy task by task id %d to update builder deploy status, error: %w", data.TaskId, err)
		}

		if task.Deploy == nil {
			slog.Warn("deploy does not exist and system will skip updating builder deploy status", slog.Any("task_id", data.TaskId))
			return nil
		}

		if event.EventTime < task.UpdatedAt.Unix() {
			return nil
		}
		var status int
		switch data.Status {
		case string(v1alpha1.WorkflowRunning):
			status = scheduler.BuildInProgress
			task.Deploy.Status = common.Building
		case string(v1alpha1.WorkflowSucceeded):
			if task.Deploy.Status != common.Building {
				slog.Warn("deploy status is not building, skip setting build success status")
				return nil
			}
			status = scheduler.BuildSucceed
			task.Deploy.ImageID = data.ImagetPath
			task.Deploy.Status = common.BuildSuccess
		case string(v1alpha1.WorkflowFailed):
			if task.Deploy.Status != common.Building {
				slog.Warn("deploy status is not building, skip setting build failed status in imagebuilderwebhook",
					slog.Any("task_id", task.ID), slog.Any("deploy_id", task.Deploy.ID), slog.Any("deploy_status", task.Deploy.Status))
				return nil
			}
			status = scheduler.BuildFailed
			task.Deploy.Status = common.BuildFailed
		case string(v1alpha1.WorkflowError):
			if task.Deploy.Status != common.Building {
				slog.Warn("deploy status is not building, skip setting build error status")
				return nil
			}
			status = scheduler.BuildFailed
		default:
			return nil
		}

		if task.Status == scheduler.Cancelled {
			return nil
		}

		if task.Status != scheduler.BuildInQueue && status <= task.Status {
			return nil
		}

		task.Message = data.Message
		task.Status = status

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.store.UpdateInTx(ctx, []string{"status", "image_id"}, []string{"status", "message"}, task.Deploy, task); err != nil {
			slog.Error("failed to change deploy status to `BuildSuccess`", "error", err)
			return err
		}

	}
	return nil
}
