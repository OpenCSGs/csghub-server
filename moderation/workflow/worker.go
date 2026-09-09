package workflow

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

func RegisterWorker(cfg *config.Config, wfClient temporal.Client) {
	wfWorker := wfClient.NewWorker(common.RepoFullCheckQueue, worker.Options{})
	wfWorker.RegisterWorkflowWithOptions(RepoFullCheckWorkflow, workflow.RegisterOptions{
		Name: common.RepoFullCheckWorkflowName,
	})
	wfWorker.RegisterActivity(activity.RepoSensitiveCheckPending)
	wfWorker.RegisterActivity(activity.GenRepoFileList)
	wfWorker.RegisterActivity(activity.CheckRepoFiles)
	wfWorker.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Comment media moderation poll workflow on a dedicated queue so it does
	// not block repo full-check activities.
	activity.ConfigureCommentMediaActivities(cfg)
	pollWorker := wfClient.NewWorker(common.PollCommentMediaQueue, worker.Options{})
	pollWorker.RegisterWorkflowWithOptions(PollCommentMediaModerationWorkflow, workflow.RegisterOptions{
		Name: common.PollCommentMediaWorkflowName,
	})
	pollWorker.RegisterActivity(activity.PollCommentMediaModerationResultActivity)
	pollWorker.RegisterActivity(activity.FinalizeCommentMediaModerationActivity)
	pollWorker.RegisterActivity(activity.DeleteCommentAfterFinalizeFailureActivity)
	pollWorker.RegisterActivity(activity.SendApprovedCommentNotificationActivity)
}
