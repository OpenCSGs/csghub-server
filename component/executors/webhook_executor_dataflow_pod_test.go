package executors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestDataflowPodExecutor(store database.ArgoWorkFlowStore) WebHookExecutor {
	executor := &dataflowPodExecutorImpl{
		store: store,
	}
	return executor
}

func TestDataflowPodExecutor_ProcessEvent(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	t.Run("update cluster node and dag tasks", func(t *testing.T) {
		existingDagTasks := `{"task1": {"status": "running", "start_time": "2024-01-01T00:00:00Z"}}`
		oldWF := &database.ArgoWorkflow{
			ID:          1,
			TaskId:      "task-123",
			ClusterNode: "old-node",
			DagTasks:    existingDagTasks,
		}

		newDagTasks := `{"task2": {"status": "succeeded", "start_time": "2024-01-01T01:00:00Z"}}`
		newWF := database.ArgoWorkflow{
			TaskId:      "task-123",
			ClusterNode: "new-node",
			DagTasks:    newDagTasks,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-123").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.ID == 1 &&
				wf.TaskId == "task-123" &&
				wf.ClusterNode == "new-node" &&
				len(wf.DagTasks) > 0
		})).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("update cluster node only", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:          2,
			TaskId:      "task-456",
			ClusterNode: "old-node",
			DagTasks:    "",
		}

		newWF := database.ArgoWorkflow{
			TaskId:      "task-456",
			ClusterNode: "new-node",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-456").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.ID == 2 &&
				wf.TaskId == "task-456" &&
				wf.ClusterNode == "new-node" &&
				wf.DagTasks == ""
		})).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("update dag tasks with empty existing", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:          3,
			TaskId:      "task-789",
			ClusterNode: "node-1",
			DagTasks:    "",
		}

		newDagTasks := `{"task1": {"status": "running"}}`
		newWF := database.ArgoWorkflow{
			TaskId:   "task-789",
			DagTasks: newDagTasks,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-789").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			var dagMap map[string]interface{}
			err := json.Unmarshal([]byte(wf.DagTasks), &dagMap)
			if err != nil {
				return false
			}
			task1, ok := dagMap["task1"]
			return ok &&
				wf.ID == 3 &&
				wf.TaskId == "task-789" &&
				wf.ClusterNode == "node-1" &&
				task1.(map[string]interface{})["status"] == "running"
		})).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("merge dag tasks", func(t *testing.T) {
		existingDagTasks := `{"task1": {"status": "running"}, "task2": {"status": "pending"}}`
		oldWF := &database.ArgoWorkflow{
			ID:       4,
			TaskId:   "task-merge",
			DagTasks: existingDagTasks,
		}

		newDagTasks := `{"task2": {"status": "succeeded"}, "task3": {"status": "running"}}`
		newWF := database.ArgoWorkflow{
			TaskId:   "task-merge",
			DagTasks: newDagTasks,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-merge").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			var dagMap map[string]interface{}
			err := json.Unmarshal([]byte(wf.DagTasks), &dagMap)
			if err != nil {
				return false
			}
			task1, ok1 := dagMap["task1"]
			task2, ok2 := dagMap["task2"]
			task3, ok3 := dagMap["task3"]
			return ok1 && ok2 && ok3 &&
				task1.(map[string]interface{})["status"] == "running" &&
				task2.(map[string]interface{})["status"] == "succeeded" &&
				task3.(map[string]interface{})["status"] == "running"
		})).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("workflow not found", func(t *testing.T) {
		newWF := database.ArgoWorkflow{
			TaskId:      "task-not-exist",
			ClusterNode: "node-1",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-not-exist").Return(nil, errors.New("not found"))

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: []byte("invalid json"),
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		executor := NewTestDataflowPodExecutor(mockStore)
		err := executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal dataflow pod event data")
	})

	t.Run("invalid existing dag_tasks json", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:       5,
			TaskId:   "task-bad-existing",
			DagTasks: "invalid json",
		}

		newDagTasks := `{"task1": {"status": "running"}}`
		newWF := database.ArgoWorkflow{
			TaskId:   "task-bad-existing",
			DagTasks: newDagTasks,
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-bad-existing").Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal existing dag_tasks map string")
	})

	t.Run("invalid new dag_tasks json", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:       6,
			TaskId:   "task-bad-new",
			DagTasks: `{"task1": {"status": "running"}}`,
		}

		newWF := database.ArgoWorkflow{
			TaskId:   "task-bad-new",
			DagTasks: "invalid new json",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-bad-new").Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal new dag_tasks map string")
	})

	t.Run("update error", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:          7,
			TaskId:      "task-update-error",
			ClusterNode: "old-node",
			DagTasks:    "",
		}

		newWF := database.ArgoWorkflow{
			TaskId:      "task-update-error",
			ClusterNode: "new-node",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-update-error").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.Anything).Return(nil, errors.New("update failed"))

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to update dataflow workflow dag_tasks")
	})

	t.Run("RunnerDataflowPodDelete event", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:          8,
			TaskId:      "task-pod-delete",
			ClusterNode: "node-1",
			DagTasks:    "",
		}

		newWF := database.ArgoWorkflow{
			TaskId:      "task-pod-delete",
			ClusterNode: "node-2",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodDelete,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-pod-delete").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.Anything).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("no fields to update", func(t *testing.T) {
		oldWF := &database.ArgoWorkflow{
			ID:          9,
			TaskId:      "task-no-update",
			ClusterNode: "node-1",
			DagTasks:    "",
		}

		newWF := database.ArgoWorkflow{
			TaskId: "task-no-update",
		}

		data, err := json.Marshal(newWF)
		require.NoError(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerDataflowPodUpdate,
				EventTime: now.Unix(),
			},
			Data: data,
		}

		mockStore := mockdb.NewMockArgoWorkFlowStore(t)
		mockStore.EXPECT().FindByTaskID(ctx, "task-no-update").Return(oldWF, nil)
		mockStore.EXPECT().UpdateWorkFlowByTaskID(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.ClusterNode == "node-1" && wf.DagTasks == ""
		})).Return(oldWF, nil)

		executor := NewTestDataflowPodExecutor(mockStore)
		err = executor.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})
}

func TestNewDataflowPodExecutor(t *testing.T) {
	cfg := &config.Config{}

	t.Run("success", func(t *testing.T) {
		executor, err := NewDataflowPodExecutor(cfg)
		require.NoError(t, err)
		require.NotNil(t, executor)
	})
}
