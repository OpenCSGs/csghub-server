package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type DataflowPodExecutor interface {
}

type dataflowPodExecutorImpl struct {
	store database.ArgoWorkFlowStore
}

var _ DataflowPodExecutor = (*dataflowPodExecutorImpl)(nil)
var _ WebHookExecutor = (*dataflowPodExecutorImpl)(nil)

func NewDataflowPodExecutor(config *config.Config) (DataflowPodExecutor, error) {
	executor := &dataflowPodExecutorImpl{
		store: database.NewArgoWorkFlowStore(),
	}

	err := RegisterWebHookExecutor(types.RunnerDataflowPodUpdate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register dataflow pod update executor: %w", err)
	}

	err = RegisterWebHookExecutor(types.RunnerDataflowPodDelete, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register dataflow pod delete executor: %w", err)
	}

	return executor, nil
}

func (h *dataflowPodExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	var newWF database.ArgoWorkflow
	err := json.Unmarshal(event.Data, &newWF)
	if err != nil {
		return fmt.Errorf("failed to unmarshal dataflow pod event data: %w", err)
	}

	slog.InfoContext(ctx, "dataflow_pod_webhook_event", slog.Any("event_type", event.EventType), slog.Any("newWF", newWF))

	oldwf, err := h.store.FindByTaskID(ctx, newWF.TaskId)
	if err != nil {
		slog.WarnContext(ctx, "dataflow workflow not exists and skip pod update", slog.Any("task_id", newWF.TaskId))
		return nil
	}

	if len(newWF.ClusterNode) > 0 {
		oldwf.ClusterNode = newWF.ClusterNode
	}

	if len(newWF.DagTasks) > 0 {
		var existingMap map[string]interface{}
		if oldwf.DagTasks != "" {
			err := json.Unmarshal([]byte(oldwf.DagTasks), &existingMap)
			if err != nil {
				return fmt.Errorf("failed to unmarshal existing dag_tasks map string %s to map error: %w", oldwf.DagTasks, err)
			}
		} else {
			existingMap = make(map[string]interface{})
		}
		var newMap map[string]interface{}
		err = json.Unmarshal([]byte(newWF.DagTasks), &newMap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal new dag_tasks map string %s to map error: %w", newWF.DagTasks, err)
		}
		for k, v := range newMap {
			existingMap[k] = v
		}
		merged, err := json.Marshal(existingMap)
		if err != nil {
			return fmt.Errorf("failed to marshal merged dag_tasks map string: %w", err)
		}
		oldwf.DagTasks = string(merged)
	}

	_, err = h.store.UpdateWorkFlowByTaskID(ctx, *oldwf)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update dataflow workflow dag_tasks",
			slog.Any("task_id", newWF.TaskId),
			slog.Any("dag_tasks", oldwf.DagTasks),
			slog.Any("err", err))
		return fmt.Errorf("failed to update dataflow workflow dag_tasks: %w", err)
	}

	return nil
}
