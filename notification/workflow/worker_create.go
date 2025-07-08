package workflow

import (
	"log/slog"

	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/notification/notifychannel/worker"

	// blank import to register workers via their init() function.
	_ "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow"
	_ "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow"
)

func createWorker(cfg *config.Config, workflowClient temporal.Client) {
	for name, creator := range worker.GetWorkerCreators() {
		slog.Info("Starting worker for notification channel", "channel", name)
		creator(cfg, workflowClient)
	}
	extendWorker(cfg, workflowClient)
}
