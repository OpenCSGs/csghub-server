package workflow

import (
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/workflow/activity"
)

const WorkflowUserDeletionQueueName = "workflow_user_deletion_queue"

func RegisterWorker(config *config.Config, wfClient temporal.Client) {
	duWorker := wfClient.NewWorker(WorkflowUserDeletionQueueName, worker.Options{})
	duWorker.RegisterWorkflow(UserDeletionWorkflow)
	duWorker.RegisterActivity(activity.DeleteUserAndRelations)
	duWorker.RegisterWorkflow(UserSoftDeletionWorkflow)
	duWorker.RegisterActivity(activity.SoftDeleteUserAndRelations)

	extendWorker(config, wfClient)

}
