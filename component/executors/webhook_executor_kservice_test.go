package executors

import (
	"context"
	"encoding/json"
	"fmt"
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
		Status:      20,
		Message:     "msg",
		Reason:      "test",
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

	dts.EXPECT().UpdateDeploy(ctx, &database.Deploy{
		ID:      int64(1),
		Status:  event.Status,
		SvcName: event.ServiceName,
		Message: event.Message,
		Reason:  event.Reason,
	}).Return(nil)

	exec := NewTestKServiceExecutor(cfg, dts)

	err = exec.updateDeployStatus(ctx, event)
	require.Nil(t, err)
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
	t.Run("notebook", func(t *testing.T) {
		deploy := &database.Deploy{
			ID:         1,
			UserUUID:   "user1",
			DeployName: "deploy1",
			Type:       types.NotebookType,
			GitPath:    "ns/n",
		}
		payload, url := buildDeployNotification(deploy)
		require.Equal(t, payload["deploy_name"], deploy.DeployName)
		require.Equal(t, payload["deploy_id"], deploy.ID)
		require.Equal(t, payload["git_path"], deploy.GitPath)
		require.Equal(t, url, fmt.Sprintf("/notebooks/%d", deploy.ID))
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
