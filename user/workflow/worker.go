package workflow

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/builder/temporal"
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
	})
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error:%w", err)
	}
	wfWorker = worker.New(wfClient, WorkflowUserDeletionQueueName, worker.Options{})
	wfWorker.RegisterWorkflow(UserDeletionWorkflow)
	wfWorker.RegisterActivity(activity.DeleteUserAndRelations)
	wfWorker.RegisterWorkflow(UserSoftDeletionWorkflow)
	wfWorker.RegisterActivity(activity.SoftDeleteUserAndRelations)

	return wfWorker.Start()
}

// RegisterWorker registers the user-deletion worker on a shared temporal
// client. The caller is responsible for starting the shared client.
func RegisterWorker(_ *config.Config, wfClient temporal.Client) {
	wfWorker := wfClient.NewWorker(WorkflowUserDeletionQueueName, worker.Options{})
	wfWorker.RegisterWorkflow(UserDeletionWorkflow)
	wfWorker.RegisterActivity(activity.DeleteUserAndRelations)
	wfWorker.RegisterWorkflow(UserSoftDeletionWorkflow)
	wfWorker.RegisterActivity(activity.SoftDeleteUserAndRelations)
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
