package workflow

import (
	"fmt"

	temporalActivity "go.temporal.io/sdk/activity"
	temporalWorker "go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	"opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow/activity"
	"opencsg.com/csghub-server/notification/notifychannel/worker"
)

const (
	GetEmailFromNotificationSettingActivity string = "GetEmailFromNotificationSettingActivity"
	GetEmailFromUserActivity                string = "GetEmailFromUserActivity"
	SendEmailBatchActivity                  string = "SendEmailBatchActivity"
)

func init() {
	worker.RegisterWorker("email", createBroadcastEmailWorker)
}

func createBroadcastEmailWorker(config *config.Config, temporalClient temporal.Client) {
	storage := database.NewNotificationStore()
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userSvcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	emailService := emailclient.NewEmailService(config)

	act := activity.NewBroadcastEmailActivity(storage, userSvcClient, emailService)
	beWorker := temporalClient.NewWorker(WorkflowBroadcastEmailQueueName, temporalWorker.Options{})
	beWorker.RegisterWorkflow(BroadcastEmailWorkflow)
	beWorker.RegisterActivityWithOptions(act.GetEmailFromNotificationSettingActivity, temporalActivity.RegisterOptions{Name: GetEmailFromNotificationSettingActivity})
	beWorker.RegisterActivityWithOptions(act.GetEmailFromUserActivity, temporalActivity.RegisterOptions{Name: GetEmailFromUserActivity})
	beWorker.RegisterActivityWithOptions(act.SendEmailBatchActivity, temporalActivity.RegisterOptions{Name: SendEmailBatchActivity})
}
