package workflow

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/common/config"
)

const HandlePushQueueName = "workflow_handle_push_queue"

var (
	wfWorker worker.Worker
	wfClient client.Client
)

func StartWorker(config *config.Config) error {
	var err error
	wfClient, err = client.Dial(client.Options{
		HostPort: config.WorkFLow.Endpoint,
	})
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error:%w", err)
	}
	wfWorker = worker.New(wfClient, HandlePushQueueName, worker.Options{})
	wfWorker.RegisterWorkflow(HandlePushWorkflow)
	wfWorker.RegisterActivity(activity.WatchSpaceChange)
	wfWorker.RegisterActivity(activity.WatchRepoRelation)
	wfWorker.RegisterActivity(activity.SetRepoUpdateTime)
	wfWorker.RegisterActivity(activity.UpdateRepoInfos)
	wfWorker.RegisterActivity(activity.SensitiveCheck)

	return wfWorker.Start()
}

func StopWorker() {
	if wfWorker != nil {
		wfWorker.Stop()
	}
	if wfClient != nil {
		wfClient.Close()
	}
}

func GetWorkflowClient() client.Client {
	return wfClient
}
