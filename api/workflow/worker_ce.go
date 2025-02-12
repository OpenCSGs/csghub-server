//go:build !ee && !saas

package workflow

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/component/callback"
)

const HandlePushQueueName = "workflow_handle_push_queue"

var activities activity.Activities

func StartWorkflow(cfg *config.Config) error {
	gitcallback, err := callback.NewGitCallback(cfg)
	if err != nil {
		return err
	}
	recom, err := component.NewRecomComponent(cfg)
	if err != nil {
		return err
	}
	gitserver, err := git.NewGitServer(cfg)
	if err != nil {
		return err
	}
	multisync, err := component.NewMultiSyncComponent(cfg)
	if err != nil {
		return err
	}
	client, err := temporal.NewClient(client.Options{
		HostPort: cfg.WorkFLow.Endpoint,
	}, "csghub-api")
	if err != nil {
		return fmt.Errorf("unable to create workflow client, error: %w", err)
	}
	if err != nil {
		return err
	}
	rftScanner, err := component.NewRuntimeArchitectureComponent(cfg)
	if err != nil {
		return err
	}
	return StartWorkflowDI(
		cfg, gitcallback, recom,
		gitserver, multisync, database.NewSyncClientSettingStore(), client,
		rftScanner,
	)
}

func StartWorkflowDI(
	cfg *config.Config,
	callback callback.GitCallbackComponent,
	recom component.RecomComponent,
	gitServer gitserver.GitServer,
	multisync component.MultiSyncComponent,
	syncClientSetting database.SyncClientSettingStore,
	temporalClient temporal.Client,
	rftScanner component.RuntimeArchitectureComponent,
) error {
	worker := temporalClient.NewWorker(HandlePushQueueName, worker.Options{})
	act := activity.NewActivities(cfg, callback, recom, gitServer, multisync, syncClientSetting, rftScanner)
	worker.RegisterActivity(act)

	worker.RegisterWorkflow(HandlePushWorkflow)
	worker.RegisterWorkflow(RuntimeFrameworkWorkflow)

	RegisterCronWorker(cfg, temporalClient, act)
	err := RegisterCronJobs(cfg, temporalClient)
	if err != nil {
		return fmt.Errorf("failed to register cron jobs:  %w", err)
	}

	err = temporalClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start worker:  %w", err)
	}
	return nil

}
