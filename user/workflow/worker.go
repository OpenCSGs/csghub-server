package workflow

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/workflow/activity"
)

const WorkflowUserDeletionQueueName = "workflow_user_deletion_queue"

var wfWorker worker.Worker
var wfClient client.Client

func StartWorker(config *config.Config) error {
	var err error
	wfClient, err = client.Dial(client.Options{
		HostPort: config.WorkFLow.Endpoint,
		Logger:   slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error:%w", err)
	}
	wfWorker = worker.New(wfClient, WorkflowUserDeletionQueueName, worker.Options{})
	wfWorker.RegisterWorkflow(UserDeletionWorkflow)
	wfWorker.RegisterActivity(activity.DeleteUserAndRelations)

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
