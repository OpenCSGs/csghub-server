package executors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type KServiceExecutor interface {
	updateDeployStatus(ctx context.Context, event *types.ServiceEvent) error
}

type kserviceExecutorImpl struct {
	cfg                   *config.Config
	deployTaskStore       database.DeployTaskStore
	notificationSvcClient rpc.NotificationSvcClient
}

var _ KServiceExecutor = (*kserviceExecutorImpl)(nil)
var _ WebHookExecutor = (*kserviceExecutorImpl)(nil)

func NewKServiceExecutor(config *config.Config) (KServiceExecutor, error) {
	notificationSvcClient := rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port), rpc.AuthWithApiKey(config.APIToken))
	executor := &kserviceExecutorImpl{
		cfg:                   config,
		deployTaskStore:       database.NewDeployTaskStore(),
		notificationSvcClient: notificationSvcClient,
	}
	// register the kservice executor for webhook callback func ProcessEvent
	err := RegisterWebHookExecutor(types.RunnerServiceCreate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register kservice create executor: %w", err)
	}
	err = RegisterWebHookExecutor(types.RunnerServiceChange, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register kservice update executor: %w", err)
	}
	err = RegisterWebHookExecutor(types.RunnerServiceStop, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register kservice stop executor: %w", err)
	}
	return executor, nil
}

func (k *kserviceExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	slog.Info("deploy_event_received", slog.Any("event", event))

	kserviceEvent := &types.ServiceEvent{}

	err := json.Unmarshal(event.Data, kserviceEvent)
	if err != nil {
		return fmt.Errorf("failed to unmarshal webhook kservice event error: %w", err)
	}

	err = k.updateDeployStatus(ctx, kserviceEvent)
	if err != nil {
		return fmt.Errorf("failed to update deploy status in webhook error: %w", err)
	}

	return nil
}

func (k *kserviceExecutorImpl) updateDeployStatus(ctx context.Context, event *types.ServiceEvent) error {
	deployTask, err := k.deployTaskStore.GetDeployTask(ctx, event.TaskID)
	if err != nil {
		return fmt.Errorf("failed to get deploy task by task id %d in webhook error: %w", event.TaskID, err)
	}

	lastTask, err := k.deployTaskStore.GetLastTaskByType(ctx, deployTask.DeployID, deployTask.TaskType)
	if err != nil {
		return fmt.Errorf("failed to get last deploy task by deploy id %d in webhook error: %w", deployTask.DeployID, err)
	}

	if lastTask.ID != deployTask.ID {
		slog.Warn("skip update deploy status as last task is not current task in webhook",
			slog.Any("event", event),
			slog.Int64("last_task_id", lastTask.ID),
			slog.Any("current_task_id", deployTask.ID),
		)
		return nil
	}

	deploy, err := k.deployTaskStore.GetDeployBySvcName(ctx, event.ServiceName)
	if errors.Is(err, sql.ErrNoRows) {
		slog.Warn("skip update deploy status as no deploy found by svc name in webhook",
			slog.Any("event", event))
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get deploy by service name %s in webhook", event.ServiceName)
	}

	if deploy.Status == common.Stopped && event.Status == common.DeployFailed {
		slog.Warn("do not allow update deploy which has been stopped to failed in webhook", slog.Any("event", event))
		return nil
	}

	if deploy.Status == common.Deleted {
		slog.Warn("do not allow update deploy which has been deleted in webhook", slog.Any("event", event))
		return nil
	}

	oldStatus := deploy.Status
	deploy.Status = event.Status
	deploy.Message = event.Message
	deploy.Reason = event.Reason
	deploy.Endpoint = event.Endpoint
	if len(event.ClusterNode) > 0 && !slices.Contains(strings.Split(deploy.ClusterNode, ","), event.ClusterNode) {
		if len(deploy.ClusterNode) > 0 {
			deploy.ClusterNode += ","
		}
		deploy.ClusterNode += fmt.Sprintf("%s%s", deploy.ClusterNode, event.ClusterNode)
	}
	if len(event.QueueName) > 0 {
		deploy.QueueName = event.QueueName
	}
	err = k.deployTaskStore.UpdateDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("failed to update deploy %s status %d in webhook error: %w", event.ServiceName, event.Status, err)
	}

	if event.Status == common.Running && oldStatus != common.Running {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := k.sendNotification(ctx, deploy)
			if err != nil {
				slog.Error("failed to send notification", slog.Any("err", err))
			}
		}()
	}

	return nil
}

func (k *kserviceExecutorImpl) sendNotification(ctx context.Context, deploy *database.Deploy) error {
	payload, url := buildDeployNotification(deploy)

	msg := types.NotificationMessage{
		UserUUIDs:        []string{deploy.UserUUID},
		NotificationType: types.NotificationDeploymentManagement,
		ClickActionURL:   url,
		Template:         string(types.MessageScenarioDeployment),
		Payload:          payload,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}
	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioDeployment,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := k.cfg.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = k.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}

func buildDeployNotification(deploy *database.Deploy) (payload map[string]any, url string) {
	payload = map[string]any{
		"deploy_name": deploy.DeployName,
		"deploy_id":   deploy.ID,
		"git_path":    deploy.GitPath,
	}

	switch deploy.Type {
	case types.SpaceType:
		payload["deploy_type"] = "space"
		url = fmt.Sprintf("/spaces/%s", deploy.GitPath)
	case types.InferenceType:
		payload["deploy_type"] = "inference"
		url = fmt.Sprintf("/endpoints/%s/%d", deploy.GitPath, deploy.ID)
	case types.FinetuneType:
		payload["deploy_type"] = "finetune"
		url = fmt.Sprintf("/finetune/%s/%s/%d", deploy.GitPath, deploy.DeployName, deploy.ID)
	case types.EvaluationType:
		payload["deploy_type"] = "evaluation"
		url = ""
	case types.ServerlessType:
		payload["deploy_type"] = "serverless"
		url = ""
	default:
		payload = map[string]any{}
		url = ""
	}
	return
}
