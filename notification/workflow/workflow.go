package workflow

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
)

var (
	workflowClient temporal.Client
)

func StartWorkflow(cfg *config.Config) error {
	var err error
	workflowClient, err = temporal.NewClient(client.Options{
		HostPort: cfg.WorkFLow.Endpoint,
		Logger:   log.NewStructuredLogger(slog.Default()),
	}, "csghub-notification")
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error: %w", err)
	}
	// create worker for each channel
	createWorker(cfg, workflowClient)

	return workflowClient.Start()
}

func GetWorkflowClient() temporal.Client {
	return workflowClient
}
