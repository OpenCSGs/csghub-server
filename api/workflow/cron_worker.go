package workflow

import (
	"context"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/common/config"
)

const (
	AlreadyScheduledMessage = "schedule with this ID is already registered"
	CronJobQueueName        = "workflow_cron_queue"
)

func RegisterCronJobs(config *config.Config) error {
	var err error
	if wfClient == nil {
		wfClient, err = client.Dial(client.Options{
			HostPort: config.WorkFLow.Endpoint,
		})
		if err != nil {
			return fmt.Errorf("unable to create workflow client, error:%w", err)
		}
	}

	if !config.Saas {
		_, err = wfClient.ScheduleClient().Create(context.Background(), client.ScheduleOptions{
			ID: "sync-as-client-schedule",
			Spec: client.ScheduleSpec{
				CronExpressions: []string{config.CronJob.SyncAsClientCronExpression},
			},
			Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
			Action: &client.ScheduleWorkflowAction{
				ID:        "sync-as-client-workflow",
				TaskQueue: CronJobQueueName,
				Workflow:  SyncAsClientWorkflow,
				Args:      []interface{}{config},
			},
		})
		if err != nil && err.Error() != AlreadyScheduledMessage {
			return fmt.Errorf("unable to create schedule, error:%w", err)
		}
	}

	_, err = wfClient.ScheduleClient().Create(context.Background(), client.ScheduleOptions{
		ID: "calc-recom-score-schedule",
		Spec: client.ScheduleSpec{
			CronExpressions: []string{config.CronJob.CalcRecomScoreCronExpression},
		},
		Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		Action: &client.ScheduleWorkflowAction{
			ID:        "calc-recom-score-workflow",
			TaskQueue: CronJobQueueName,
			Workflow:  CalcRecomScoreWorkflow,
			Args:      []interface{}{config},
		},
	})
	if err != nil && err.Error() != AlreadyScheduledMessage {
		return fmt.Errorf("unable to create schedule, error:%w", err)
	}

	return nil
}

func StartCronWorker(config *config.Config) error {
	var err error
	if wfClient == nil {
		wfClient, err = client.Dial(client.Options{
			HostPort: config.WorkFLow.Endpoint,
		})
		if err != nil {
			return fmt.Errorf("unable to create workflow client, error:%w", err)
		}
	}
	wfWorker = worker.New(wfClient, CronJobQueueName, worker.Options{})
	if !config.Saas {
		wfWorker.RegisterWorkflow(SyncAsClientWorkflow)
		wfWorker.RegisterActivity(activity.SyncAsClient)
	}
	wfWorker.RegisterWorkflow(CalcRecomScoreWorkflow)
	wfWorker.RegisterActivity(activity.CalcRecomScore)

	return wfWorker.Start()
}
