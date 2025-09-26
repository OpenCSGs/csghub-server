package executors

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type ArgoWorkflowExecutor interface {
}

type argoWorkflowExecutorImpl struct {
	store database.ArgoWorkFlowStore
}

func NewArgoWorkflowExecutor(config *config.Config) (ArgoWorkflowExecutor, error) {
	executor := &argoWorkflowExecutorImpl{
		store: database.NewArgoWorkFlowStore(),
	}

	err := RegisterWebHookExecutor(types.RunnerWorkflowCreate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register argo workflow create executor: %w", err)
	}

	err = RegisterWebHookExecutor(types.RunnerWorkflowChange, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register argo workflow failure executor: %w", err)
	}

	return executor, nil
}

func (h *argoWorkflowExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	var wf database.ArgoWorkflow
	err := json.Unmarshal(event.Data, &wf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	switch event.EventType {
	case types.RunnerWorkflowCreate:
		oldwf, err := h.store.FindByTaskID(ctx, wf.TaskId)
		if err == nil && oldwf.ID != 0 {
			// already exists
			return nil
		}
		_, err = h.store.CreateWorkFlow(ctx, wf)
		if err != nil {
			return fmt.Errorf("failed to create argo workflow: %w", err)
		}
	case types.RunnerWorkflowChange:
		_, err := h.store.UpdateWorkFlowByTaskID(ctx, wf)
		if err != nil {
			return fmt.Errorf("failed to update argo workflow: %w", err)
		}

	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}

	return nil
}
