package executors

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestImageBuilderExecutor_ProcessEvent(t *testing.T) {
	deployId := int64(1)
	taskId := int64(1)
	lastCommitID := "812e1575865e1bc394351b38ff16f00737ef0bb5"

	t.Run("WorkflowPending", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			ID: deployId,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
			Status:   scheduler.BuildPending,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowPending),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.Equal(t, scheduler.BuildPending, task.Status)
	})

	t.Run("WorkflowRunning", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			Status: common.BuildInQueue,
			ID:     deployId,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
			Status:   scheduler.BuildInQueue,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowRunning),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.Equal(t, scheduler.BuildInProgress, task.Status)
	})

	t.Run("WorkflowSucceeded", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			ID:     deployId,
			Status: common.Building,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowSucceeded),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.Equal(t, scheduler.BuildSucceed, task.Status)
		require.Equal(t, common.BuildSuccess, task.Deploy.Status)
	})

	t.Run("WorkflowFailed", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			ID:     deployId,
			Status: common.Building,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowFailed),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.Equal(t, scheduler.BuildFailed, task.Status)
		require.Equal(t, common.BuildFailed, task.Deploy.Status)
	})

	t.Run("WorkflowSucceeded not building", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			ID: deployId,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowSucceeded),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.NotEqual(t, scheduler.BuildSucceed, task.Status)
		require.NotEqual(t, common.BuildSuccess, task.Deploy.Status)
	})

	t.Run("WorkflowFailed not building", func(t *testing.T) {
		db := tests.InitTestDB()
		defer db.Close()
		mockDb := database.NewDeployTaskStoreWithDB(db)
		executor := imagebuilderExecutorImpl{
			store: mockDb,
		}
		err := mockDb.CreateDeploy(context.Background(), &database.Deploy{
			ID: deployId,
		})
		require.Nil(t, err)
		err = mockDb.CreateDeployTask(context.Background(), &database.DeployTask{
			ID:       taskId,
			DeployID: deployId,
		})
		require.Nil(t, err)
		data := types.ImageBuilderEvent{
			DeployId:   strconv.FormatInt(deployId, 10),
			TaskId:     taskId,
			Status:     string(v1alpha1.WorkflowFailed),
			Message:    "",
			ImagetPath: lastCommitID,
		}
		jsonData, err := json.Marshal(data)
		require.Nil(t, err)

		event := &types.WebHookRecvEvent{
			WebHookHeader: types.WebHookHeader{
				EventType: types.RunnerBuilderChange,
				EventTime: time.Now().Unix(),
				DataType:  types.WebHookDataTypeObject,
			},
			Data: jsonData,
		}
		err = executor.ProcessEvent(context.Background(), event)
		require.Nil(t, err)

		task, err := mockDb.GetDeployTask(context.Background(), taskId)
		require.Nil(t, err)
		require.NotEqual(t, scheduler.BuildFailed, task.Status)
		require.NotEqual(t, common.BuildFailed, task.Deploy.Status)
	})

}
