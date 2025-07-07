package workflow

import (
	"fmt"

	temporalActivity "go.temporal.io/sdk/activity"
	temporalWorker "go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	activity "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow/activity"
	"opencsg.com/csghub-server/notification/notifychannel/worker"
)

const (
	WorkflowBroadcastInternalMessageQueueName string = "workflow_broadcast_internal_message_queue"
	InsertUserMessageBatchActivity            string = "InsertUserMessageBatchActivity"
	LogUserMessageFailuresActivity            string = "LogUserMessageFailuresActivity"
)

func init() {
	worker.RegisterWorker("internalmsg", createBroadcastInternalMessageWorker)
}

func createBroadcastInternalMessageWorker(config *config.Config, temporalClient temporal.Client) {
	storage := database.NewNotificationStore()
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userSvcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))

	act := activity.NewBroadcastMessageActivity(storage, userSvcClient)
	beWorker := temporalClient.NewWorker(WorkflowBroadcastInternalMessageQueueName, temporalWorker.Options{})
	beWorker.RegisterWorkflow(BroadcastInternalMessageWorkflow)
	beWorker.RegisterActivityWithOptions(act.InsertUserMessageBatchActivity, temporalActivity.RegisterOptions{Name: InsertUserMessageBatchActivity})
	beWorker.RegisterActivityWithOptions(act.LogUserMessageFailuresActivity, temporalActivity.RegisterOptions{Name: LogUserMessageFailuresActivity})
}
