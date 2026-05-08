package executors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestDataflowExecutor(store database.ArgoWorkFlowStore) WebHookExecutor {
	executor := &dataflowExecutorImpl{
		store: store,
	}
	return executor
}

func TestDataflowExecutor_ProcessEvent(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	t.Run("RunnerDataflowChange success", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId:      "task-123",
			Status:      v1alpha1.WorkflowRunning,
			Reason:      "test reason",
			Namespace:   "default",
			StartTime:   now,
			EndTime:     now.Add(1 * time.Hour),
			QueueName:   "queue-1",
			ClusterNode: "node-1",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowChange,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:          1,
			TaskId:      "task-123",
			Status:      v1alpha1.WorkflowPending,
			Reason:      "",
			Namespace:   "",
			StartTime:   time.Time{},
			EndTime:     time.Time{},
			QueueName:   "",
			ClusterNode: "",
		}

		updatedWF := &database.ArgoWorkflow{
			ID:          1,
			TaskId:      "task-123",
			Status:      newWF.Status,
			Reason:      newWF.Reason,
			Namespace:   newWF.Namespace,
			StartTime:   newWF.StartTime,
			EndTime:     newWF.EndTime,
			QueueName:   newWF.QueueName,
			ClusterNode: newWF.ClusterNode,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-123").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "task-123" &&
				wf.Status == v1alpha1.WorkflowRunning &&
				wf.Reason == "test reason" &&
				wf.Namespace == "default" &&
				wf.QueueName == "queue-1" &&
				wf.ClusterNode == "node-1"
		})).Return(updatedWF, nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("RunnerDataflowChange partial update", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-456",
			Status: v1alpha1.WorkflowRunning,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowChange,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:        2,
			TaskId:    "task-456",
			Status:    v1alpha1.WorkflowPending,
			Reason:    "old reason",
			Namespace: "old-namespace",
		}

		updatedWF := &database.ArgoWorkflow{
			ID:        2,
			TaskId:    "task-456",
			Status:    v1alpha1.WorkflowRunning,
			Reason:    "old reason",
			Namespace: "old-namespace",
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-456").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, *updatedWF).Return(updatedWF, nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("RunnerDataflowDelete with pending status", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-789",
			Status: v1alpha1.WorkflowPending,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowDelete,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     3,
			TaskId: "task-789",
			Status: v1alpha1.WorkflowPending,
		}

		cancelledWF := &database.ArgoWorkflow{
			ID:     3,
			TaskId: "task-789",
			Status: types.DFCancelled,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-789").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, *cancelledWF).Return(cancelledWF, nil)
		mockStore.EXPECT().DeleteWorkFlow(ctx, oldWF.ID).Return(nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("RunnerDataflowDelete with running status", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-running",
			Status: v1alpha1.WorkflowRunning,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowDelete,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     4,
			TaskId: "task-running",
			Status: v1alpha1.WorkflowRunning,
		}

		cancelledWF := &database.ArgoWorkflow{
			ID:     4,
			TaskId: "task-running",
			Status: types.DFCancelled,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-running").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, *cancelledWF).Return(cancelledWF, nil)
		mockStore.EXPECT().DeleteWorkFlow(ctx, oldWF.ID).Return(nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("RunnerDataflowDelete with succeeded status", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-succeeded",
			Status: v1alpha1.WorkflowSucceeded,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowDelete,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     5,
			TaskId: "task-succeeded",
			Status: v1alpha1.WorkflowSucceeded,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-succeeded").Return(oldWF, nil)
		mockStore.EXPECT().DeleteWorkFlow(ctx, oldWF.ID).Return(nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("workflow not found", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-not-exist",
			Status: v1alpha1.WorkflowRunning,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowChange,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-not-exist").Return(nil, errors.New("not found"))

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowChange,
				EventTime: now.Unix(),
			},
			Data: []byte("invalid json"),
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		executor := NewTestDataflowExecutor(mockStore)
		err := executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal dataflow event data")
	})

	t.Run("unknown event type", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-unknown",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.WebHookEventType("unknown"),
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     6,
			TaskId: "task-unknown",
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-unknown").Return(oldWF, nil)

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown dataflow event type")
	})

	t.Run("RunnerDataflowChange update error", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-update-error",
			Status: v1alpha1.WorkflowRunning,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowChange,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     7,
			TaskId: "task-update-error",
			Status: v1alpha1.WorkflowPending,
		}

		updatedWF := &database.ArgoWorkflow{
			ID:     7,
			TaskId: "task-update-error",
			Status: v1alpha1.WorkflowRunning,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-update-error").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, *updatedWF).Return(nil, errors.New("update failed"))

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("RunnerDataflowDelete delete error", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId: "task-delete-error",
			Status: v1alpha1.WorkflowSucceeded,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowDelete,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		oldWF := &database.ArgoWorkflow{
			ID:     8,
			TaskId: "task-delete-error",
			Status: v1alpha1.WorkflowSucceeded,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-delete-error").Return(oldWF, nil)
		mockStore.EXPECT().DeleteWorkFlow(ctx, oldWF.ID).Return(errors.New("delete failed"))

		executor := NewTestDataflowExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})
}

func TestNewDataflowExecutor(t *testing.T) {
	cfg := &config.Config{}

	t.Run("success", func(t *testing.T) {
		executor, err := NewDataflowExecutor(cfg)
		require.NoError(t, err)
		require.NotNil(t, executor)
	})
}
