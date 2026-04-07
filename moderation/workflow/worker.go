package workflow

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

func RegisterWorker(wfClient temporal.Client) {
	wfWorker := wfClient.NewWorker(common.RepoFullCheckQueue, worker.Options{})
	wfWorker.RegisterWorkflowWithOptions(RepoFullCheckWorkflow, workflow.RegisterOptions{
		Name: common.RepoFullCheckWorkflowName,
	})
	wfWorker.RegisterActivity(activity.RepoSensitiveCheckPending)
	wfWorker.RegisterActivity(activity.GenRepoFileList)
	wfWorker.RegisterActivity(activity.CheckRepoFiles)
	wfWorker.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)
}
