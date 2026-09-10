package executors

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func workflowChangeEvent(t *testing.T, wf database.ArgoWorkflow) *types.WebHookRecvEvent {
	t.Helper()
	data, err := json.Marshal(wf)
	require.NoError(t, err)
	return &types.WebHookRecvEvent{
		WebHookHeader: types.WebHookHeader{EventType: types.RunnerWorkflowChange},
		Data:          data,
	}
}

func TestArgoWorkflowExecutorProcessChangeEvent(t *testing.T) {
	ctx := context.Background()
	eventTime := time.Now().UTC()

	t.Run("updates a newer event", func(t *testing.T) {
		store := mockdb.NewMockArgoWorkFlowStore(t)
		executor := &argoWorkflowExecutorImpl{store: store}
		incoming := database.ArgoWorkflow{
			TaskId:         "task",
			Status:         v1alpha1.WorkflowRunning,
			StatusUpdateAt: eventTime,
		}
		store.EXPECT().FindByTaskID(ctx, "task").Return(&database.ArgoWorkflow{
			TaskId:         "task",
			Status:         v1alpha1.WorkflowPending,
			StatusUpdateAt: eventTime.Add(-time.Minute),
		}, nil)
		store.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.Status == v1alpha1.WorkflowRunning && wf.StatusUpdateAt.Equal(eventTime)
		})).Return(&incoming, nil)

		require.NoError(t, executor.ProcessEvent(ctx, workflowChangeEvent(t, incoming)))
	})

	t.Run("ignores an older event", func(t *testing.T) {
		store := mockdb.NewMockArgoWorkFlowStore(t)
		executor := &argoWorkflowExecutorImpl{store: store}
		incoming := database.ArgoWorkflow{
			TaskId:         "task",
			Status:         v1alpha1.WorkflowRunning,
			StatusUpdateAt: eventTime,
		}
		store.EXPECT().FindByTaskID(ctx, "task").Return(&database.ArgoWorkflow{
			TaskId:         "task",
			Status:         v1alpha1.WorkflowSucceeded,
			StatusUpdateAt: eventTime.Add(time.Minute),
		}, nil)

		require.NoError(t, executor.ProcessEvent(ctx, workflowChangeEvent(t, incoming)))
	})

	t.Run("creates a missing workflow before applying the change", func(t *testing.T) {
		store := mockdb.NewMockArgoWorkFlowStore(t)
		executor := &argoWorkflowExecutorImpl{store: store}
		incoming := database.ArgoWorkflow{
			TaskId:         "task",
			Status:         v1alpha1.WorkflowRunning,
			StatusUpdateAt: eventTime,
		}
		store.EXPECT().FindByTaskID(ctx, "task").Return(nil, sql.ErrNoRows)
		store.EXPECT().CreateWorkFlow(ctx, incoming).Return(&incoming, nil)
		store.EXPECT().UpdateWorkFlowByTaskID(ctx, incoming).Return(&incoming, nil)

		require.NoError(t, executor.ProcessEvent(ctx, workflowChangeEvent(t, incoming)))
	})
}
