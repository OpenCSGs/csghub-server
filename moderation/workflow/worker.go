package workflow

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/workflow/activity"
)

var wfClient client.Client
var wfWorker worker.Worker

func StartWorker(config *config.Config) error {
	var err error
	wfClient, err = client.Dial(client.Options{
		HostPort: config.WorkFLow.Endpoint,
	})
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error:%w", err)
	}

	wfWorker = worker.New(wfClient, "moderation_repo_full_check_queue", worker.Options{})
	wfWorker.RegisterWorkflow(RepoFullCheckWorkflow)
	wfWorker.RegisterActivity(activity.RepoSensitiveCheckPending)
	wfWorker.RegisterActivity(activity.GenRepoFileList)
	wfWorker.RegisterActivity(activity.CheckRepoFiles)
	wfWorker.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

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
