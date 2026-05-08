package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type DataflowExecutor interface {
}

type dataflowExecutorImpl struct {
	store database.ArgoWorkFlowStore
}

var _ DataflowExecutor = (*dataflowExecutorImpl)(nil)
var _ WebHookExecutor = (*dataflowExecutorImpl)(nil)

func NewDataflowExecutor(config *config.Config) (DataflowExecutor, error) {
	executor := &dataflowExecutorImpl{
		store: database.NewArgoWorkFlowStore(),
	}

	err := RegisterWebHookExecutor(types.RunnerDataflowChange, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register dataflow change executor: %w", err)
	}

	err = RegisterWebHookExecutor(types.RunnerDataflowDelete, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register dataflow delete executor: %w", err)
	}

	return executor, nil
}

func (h *dataflowExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	var newWF database.ArgoWorkflow
	err := json.Unmarshal(event.Data, &newWF)
	if err != nil {
		return fmt.Errorf("failed to unmarshal dataflow event data: %w", err)
	}

	slog.InfoContext(ctx, "dataflow_webhook_event", slog.Any("event_type", event.EventType), slog.Any("newWF", newWF))
	oldwf, err := h.store.FindByTaskID(ctx, newWF.TaskId)
	if err != nil {
		slog.WarnContext(ctx, "dataflow workflow not exists and skip update", slog.Any("task_id", newWF.TaskId))
		return nil
	}
	if len(newWF.Status) > 0 {
		oldwf.Status = newWF.Status
	}
	if len(newWF.Reason) > 0 {
		oldwf.Reason = newWF.Reason
	}
	if len(newWF.Namespace) > 0 {
		oldwf.Namespace = newWF.Namespace
	}
	if !newWF.StartTime.IsZero() {
		oldwf.StartTime = newWF.StartTime
	}
	if !newWF.EndTime.IsZero() {
		oldwf.EndTime = newWF.EndTime
	}
	if len(newWF.QueueName) > 0 {
		oldwf.QueueName = newWF.QueueName
	}
	if len(newWF.ClusterNode) > 0 {
		oldwf.ClusterNode = newWF.ClusterNode
	}
	switch event.EventType {
	case types.RunnerDataflowChange:
		_, err = h.store.UpdateWorkFlowByTaskID(ctx, *oldwf)
		if err != nil {
			slog.ErrorContext(ctx, "failed to update dataflow workflow", slog.Any("oldwf", oldwf), slog.Any("err", err))
		}
	case types.RunnerDataflowDelete:
		if oldwf.Status == v1alpha1.WorkflowPending || oldwf.Status == v1alpha1.WorkflowRunning {
			oldwf.Status = types.DFCancelled
			_, err = h.store.UpdateWorkFlowByTaskID(ctx, *oldwf)
			if err != nil {
				slog.WarnContext(ctx, "failed to update dataflow workflow status", slog.Any("oldwf", oldwf), slog.Any("err", err))
			}
		}
		err = h.store.DeleteWorkFlow(ctx, oldwf.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to delete dataflow workflow", slog.Any("oldwf", oldwf), slog.Any("err", err))
		}
	default:
		return fmt.Errorf("unknown dataflow event type: %s", event.EventType)
	}

	return nil
}
