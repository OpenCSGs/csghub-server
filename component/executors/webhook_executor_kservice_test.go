package executors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestKServiceExecutor(config *config.Config, dts database.DeployTaskStore) KServiceExecutor {
	executor := &kserviceExecutorImpl{
		cfg:             config,
		deployTaskStore: dts,
	}
	return executor
}

func TestWebHookExecutorKService_ProcessEvent(t *testing.T) {
	ctx := context.TODO()

	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svcn",
		Status:      common.Deploying,
		Message:     "msg",
		Reason:      "test",
		ClusterNode: "node1",
		Instances: []types.Instance{
			{
				Name:   "instance1",
				Status: "running",
			},
		},
	}

	dts := mockdb.NewMockDeployTaskStore(t)
	dts.EXPECT().GetLastTaskByType(ctx, mock.Anything, mock.Anything).Return(&database.DeployTask{
		ID: int64(1),
	}, nil)

	dts.EXPECT().GetDeployTask(ctx, mock.Anything).Return(&database.DeployTask{
		ID: int64(1),
	}, nil)
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		SvcName: event.ServiceName,
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) &&
			d.Status == event.Status &&
			d.SvcName == event.ServiceName &&
			d.Message == event.Message &&
			d.Reason == event.Reason &&
			d.ClusterNode == event.ClusterNode &&
			!d.StatusUpdateAt.IsZero()
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)

	err = exec.updateDeployStatus(ctx, event)
	require.Nil(t, err)
}

func TestKServiceExecutor_updateDeployStatus_success(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Deploying,
		Message:     "deploying",
		Reason:      "triggered",
		TaskID:      100,
		Endpoint:    "http://svc-test.example.com",
		QueueName:   "queue-1",
		ClusterNode: "node1",
		Instances: []types.Instance{
			{Name: "inst1", Status: "running"},
		},
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:         int64(1),
		SvcName:    event.ServiceName,
		Status:     common.Deploying,
		ClusterNode: "",
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) &&
			d.Status == event.Status &&
			d.Message == event.Message &&
			d.Reason == event.Reason &&
			d.Endpoint == event.Endpoint &&
			d.QueueName == event.QueueName &&
			len(d.Instances) == 1 &&
			d.Instances[0].Name == "inst1" &&
			!d.StatusUpdateAt.IsZero()
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_skipWhenLastTaskDiffers(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	// Last task has a different ID, so update should be skipped
	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(999),
	}, nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_skipWhenDeployStoppedAndEventFailed(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.DeployFailed,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	// Deploy is already stopped, event says DeployFailed -> should be skipped
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Stopped,
		SvcName: event.ServiceName,
	}, nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_skipWhenDeployDeleted(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	// Deploy is deleted -> should be skipped
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Deleted,
		SvcName: event.ServiceName,
	}, nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_skipWhenNoDeployFound(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-nonexistent",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	// No deploy found by svc name (sql.ErrNoRows)
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(nil, sql.ErrNoRows)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_clearInstancesOnStopped(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Stopped,
		TaskID:      100,
		Instances: []types.Instance{
			{Name: "inst1", Status: "running"},
		},
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Running,
		SvcName: event.ServiceName,
		Instances: []types.Instance{
			{Name: "old-inst", Status: "running"},
		},
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) &&
			d.Status == common.Stopped &&
			len(d.Instances) == 0 // instances should be cleared on Stopped
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_updateEndpointAndQueueName(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Deploying,
		TaskID:      100,
		Endpoint:    "http://new-endpoint.example.com",
		QueueName:   "new-queue",
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Deploying,
		SvcName: event.ServiceName,
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) &&
			d.Endpoint == "http://new-endpoint.example.com" &&
			d.QueueName == "new-queue"
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_getDeployTaskError(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(nil, errors.New("db error"))

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get deploy task by task id")
}

func TestKServiceExecutor_updateDeployStatus_getLastTaskError(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(nil, errors.New("db error"))

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get last deploy task")
}

func TestKServiceExecutor_updateDeployStatus_getDeployBySvcNameError(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	// Return a non-sql.ErrNoRows error
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(nil, errors.New("db error"))

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get deploy by service name")
}

func TestKServiceExecutor_updateDeployStatus_updateDeployError(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Deploying,
		SvcName: event.ServiceName,
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.Anything).Return(errors.New("update error"))

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to update deploy")
}

func TestKServiceExecutor_updateDeployStatus_triggerHandleDeployRunning(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
	var wg sync.WaitGroup
	wg.Add(1)
	mockNotificationRpc.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, req *types.MessageRequest) error {
		defer wg.Done()
		return nil
	})

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:         int64(1),
		Status:     common.Deploying, // old status is Deploying, event status is Running -> should trigger
		SvcName:    event.ServiceName,
		UserUUID:   "user1",
		DeployName: "deploy1",
		Type:       types.SpaceType,
		GitPath:    "ns/n",
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.Anything).Return(nil)

	executor := &kserviceExecutorImpl{
		cfg:                   cfg,
		deployTaskStore:       dts,
		notificationSvcClient: mockNotificationRpc,
	}

	err = executor.updateDeployStatus(ctx, event)
	require.NoError(t, err)
	// Wait for the goroutine handleDeployRunning to complete
	wg.Wait()
}

func TestKServiceExecutor_updateDeployStatus_noHandleDeployRunningWhenAlreadyRunning(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Running,
		TaskID:      100,
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	// Deploy is already Running, event is also Running -> should NOT trigger handleDeployRunning
	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:      int64(1),
		Status:  common.Running, // already running
		SvcName: event.ServiceName,
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.Anything).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_appendClusterNode(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Deploying,
		TaskID:      100,
		ClusterNode: "node2",
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:          int64(1),
		Status:      common.Deploying,
		SvcName:     event.ServiceName,
		ClusterNode: "node1",
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) &&
			d.ClusterNode != "" &&
			containsClusterNode(d.ClusterNode, "node1") &&
			containsClusterNode(d.ClusterNode, "node2")
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_skipDuplicateClusterNode(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Deploying,
		TaskID:      100,
		ClusterNode: "node1",
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:          int64(1),
		Status:      common.Deploying,
		SvcName:     event.ServiceName,
		ClusterNode: "node1",
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) && d.ClusterNode == "node1"
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

func TestKServiceExecutor_updateDeployStatus_emptyClusterNodeNotAppended(t *testing.T) {
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	event := &types.ServiceEvent{
		ServiceName: "svc-test",
		Status:      common.Deploying,
		TaskID:      100,
		ClusterNode: "",
	}

	dts := mockdb.NewMockDeployTaskStore(t)

	dts.EXPECT().GetDeployTask(ctx, event.TaskID).Return(&database.DeployTask{
		ID:       int64(100),
		DeployID: int64(1),
		TaskType: common.TaskTypeDeploy,
	}, nil)

	dts.EXPECT().GetLastTaskByType(ctx, int64(1), common.TaskTypeDeploy).Return(&database.DeployTask{
		ID: int64(100),
	}, nil)

	dts.EXPECT().GetDeployBySvcName(ctx, event.ServiceName).Return(&database.Deploy{
		ID:          int64(1),
		Status:      common.Deploying,
		SvcName:     event.ServiceName,
		ClusterNode: "node1",
	}, nil)

	dts.EXPECT().UpdateDeploy(ctx, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.ID == int64(1) && d.ClusterNode == "node1"
	})).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)
	err = exec.updateDeployStatus(ctx, event)
	require.NoError(t, err)
}

// containsClusterNode checks if a node exists in a comma-separated cluster_node string
func containsClusterNode(clusterNode string, node string) bool {
	for _, n := range strings.Split(clusterNode, ",") {
		if n == node {
			return true
		}
	}
	return false
}

func TestKServiceExecutor_sendNotification(t *testing.T) {
	deploy := &database.Deploy{
		ID:         1,
		UserUUID:   "user1",
		DeployName: "deploy1",
		Type:       types.SpaceType,
		GitPath:    "ns/n",
		Status:     common.Running,
	}
	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
	var wg sync.WaitGroup
	wg.Add(1)
	mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		defer wg.Done()
		if req.Scenario != types.MessageScenarioDeployment {
			return false
		}
		var msg types.NotificationMessage
		if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
			return false
		}
		return msg.UserUUIDs[0] == deploy.UserUUID &&
			msg.NotificationType == types.NotificationDeploymentManagement &&
			msg.ClickActionURL == fmt.Sprintf("/spaces/%s", deploy.GitPath) &&
			msg.Template == string(types.MessageScenarioDeployment)
	})).Return(nil)

	config := &config.Config{}
	config.Notification.NotificationRetryCount = 3
	d := &kserviceExecutorImpl{
		cfg:                   config,
		notificationSvcClient: mockNotificationRpc,
	}
	err := d.sendNotification(context.TODO(), deploy)
	require.NoError(t, err)
	wg.Wait()
}

func TestKServiceExecutor_buildDeployNotification(t *testing.T) {
	t.Run("space", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.SpaceType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, fmt.Sprintf("/spaces/%s", deploy.GitPath))
	})
	t.Run("inference", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.InferenceType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, fmt.Sprintf("/endpoints/%s/%d", deploy.GitPath, deploy.ID))
	})
	t.Run("finetune", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.FinetuneType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, fmt.Sprintf("/finetune/%s/%s/%d", deploy.GitPath, deploy.DeployName, deploy.ID))
	})
	t.Run("evaluation", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.EvaluationType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, "")
	})
	t.Run("serverless", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.ServerlessType,
			GitPath:    "ns/n",
			Status:     common.Running,
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, "")
	})
	t.Run("unknown", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.UnknownType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload, map[string]any{})
		require.Equal(t, url, "")
	})
}
