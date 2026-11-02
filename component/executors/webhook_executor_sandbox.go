package executors

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log/slog"
// 	"strconv"

// 	"opencsg.com/csghub-server/builder/deploy/common"
// 	"opencsg.com/csghub-server/builder/store/database"
// 	"opencsg.com/csghub-server/common/config"
// 	"opencsg.com/csghub-server/common/types"
// )

// type SandboxExecutor interface {
// 	updateDeployStatus(ctx context.Context, event *types.WebHookRecvEvent) error
// }

// type sandboxExecutorImpl struct {
// 	deployTaskStore database.DeployTaskStore
// }

// func NewSandboxExecutor(config *config.Config) (SandboxExecutor, error) {
// 	executor := &sandboxExecutorImpl{
// 		deployTaskStore: database.NewDeployTaskStore(),
// 	}

// 	err := RegisterWebHookExecutor(types.RunnerSandboxCreate, executor)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to register webhook executor for sandbox create: %w", err)
// 	}
// 	err = RegisterWebHookExecutor(types.RunnerSandboxChange, executor)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to register webhook executor for sandbox change: %w", err)
// 	}

// 	err = RegisterWebHookExecutor(types.RunnerSandboxStop, executor)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to register webhook executor for sandbox stop: %w", err)
// 	}

// 	return executor, nil
// }

// var _ SandboxExecutor = (*sandboxExecutorImpl)(nil)
// var _ WebHookExecutor = (*sandboxExecutorImpl)(nil)

// func (s *sandboxExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
// 	slog.Info("sandbox_event_received", slog.Any("event", event))
// 	err := s.updateDeployStatus(ctx, event)
// 	if err != nil {
// 		return fmt.Errorf("failed to update deploy status in webhook error: %w", err)
// 	}

// 	return nil
// }

// func (s *sandboxExecutorImpl) updateDeployStatus(ctx context.Context, event *types.WebHookRecvEvent) error {
// 	sandboxEvent := &types.SandboxEvent{}

// 	err := json.Unmarshal(event.Data, sandboxEvent)
// 	if err != nil {
// 		return fmt.Errorf("failed to unmarshal webhook sandbox event error: %w", err)
// 	}

// 	taskId, err := strconv.ParseInt(sandboxEvent.DeployTaskId, 10, 64)
// 	if err != nil {
// 		return fmt.Errorf("failed to parse deploy task id %s in webhook error: %w", sandboxEvent.DeployTaskId, err)
// 	}
// 	deployTask, err := s.deployTaskStore.GetDeployTask(ctx, taskId)
// 	if err != nil {
// 		return fmt.Errorf("failed to get deploy task by task id %d in webhook error: %w", taskId, err)
// 	}

// 	lastTask, err := s.deployTaskStore.GetLastTaskByType(ctx, deployTask.DeployID, deployTask.TaskType)
// 	if err != nil {
// 		return fmt.Errorf("failed to get last deploy task by deploy id %d in webhook error: %w", deployTask.DeployID, err)
// 	}

// 	lastTask.Status = sandboxEvent.Status
// 	lastTask.Message = sandboxEvent.Message
// 	if lastTask.ID != deployTask.ID {
// 		slog.Warn("skip update deploy status as last task is not current task in webhook",
// 			slog.Any("event", sandboxEvent),
// 			slog.Int64("last_task_id", lastTask.ID),
// 			slog.Any("current_task_id", deployTask.ID),
// 		)
// 		// only update last task status
// 		if err = s.deployTaskStore.UpdateDeployTask(ctx, lastTask); err != nil {
// 			slog.ErrorContext(ctx, "failed to update sandbox task %d status %d in webhook error: %w", lastTask.ID, int(lastTask.Status), err)
// 			return fmt.Errorf("failed to update sandbox task %d status %d in webhook error: %w", lastTask.ID, int(lastTask.Status), err)
// 		}
// 		return nil
// 	}

// 	deploy, err := s.deployTaskStore.GetDeployByID(ctx, deployTask.DeployID)
// 	if err != nil {
// 		return fmt.Errorf("failed to get sandbox deploy by deploy id %d in webhook error: %w", deployTask.DeployID, err)
// 	}

// 	if deploy.Status == common.Stopped && sandboxEvent.Status == common.DeployFailed {
// 		slog.Warn("do not allow update sandbox deploy which has been stopped to failed in webhook", slog.Any("event", sandboxEvent))
// 		return nil
// 	}

// 	if deploy.Status == common.Deleted {
// 		slog.Warn("do not allow update sandbox deploy which has been deleted in webhook", slog.Any("event", sandboxEvent))
// 		return nil
// 	}

// 	if event.EventType == types.RunnerSandboxCreate && deploy.Status == common.Running {
// 		return nil
// 	}

// 	deploy.Status = sandboxEvent.Status
// 	deploy.Message = sandboxEvent.Message
// 	deploy.Reason = sandboxEvent.Reason
// 	deploy.SvcName = sandboxEvent.SandboxID

// 	err = s.deployTaskStore.UpdateDeploy(ctx, deploy)
// 	if err != nil {
// 		slog.Error("failed to update deploy %d status %d in webhook error: %w", slog.Any("deploy_id", deploy.ID), slog.Any("status", int(sandboxEvent.Status)), slog.Any("err", err))
// 		return fmt.Errorf("failed to update deploy %d status %d in webhook error: %w", deploy.ID, sandboxEvent.Status, err)
// 	}

// 	if err = s.deployTaskStore.UpdateDeployTask(ctx, lastTask); err != nil {
// 		return fmt.Errorf("failed to update sandbox task %d status %d in webhook error: %w", lastTask.ID, int(lastTask.Status), err)
// 	}
// 	return nil
// }
