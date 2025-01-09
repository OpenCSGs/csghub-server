//go:build !ee && !saas

package workflow

import (
	"context"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
)

func RegisterCronJobs(config *config.Config, temporalClient temporal.Client) error {
	var err error
	scheduler := temporalClient.GetScheduleClient()

	_, err = scheduler.Create(context.Background(), client.ScheduleOptions{
		ID: "sync-as-client-schedule",
		Spec: client.ScheduleSpec{
			CronExpressions: []string{config.CronJob.SyncAsClientCronExpression},
		},
		Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		Action: &client.ScheduleWorkflowAction{
			ID:        "sync-as-client-workflow",
			TaskQueue: CronJobQueueName,
			Workflow:  SyncAsClientWorkflow,
			Args:      []interface{}{},
		},
	})
	if err != nil && err.Error() != AlreadyScheduledMessage {
		return fmt.Errorf("unable to create schedule, error:%w", err)
	}

	_, err = scheduler.Create(context.Background(), client.ScheduleOptions{
		ID: "calc-recom-score-schedule",
		Spec: client.ScheduleSpec{
			CronExpressions: []string{config.CronJob.CalcRecomScoreCronExpression},
		},
		Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		Action: &client.ScheduleWorkflowAction{
			ID:        "calc-recom-score-workflow",
			TaskQueue: CronJobQueueName,
			Workflow:  CalcRecomScoreWorkflow,
			Args:      []interface{}{},
		},
	})
	if err != nil && err.Error() != AlreadyScheduledMessage {
		return fmt.Errorf("unable to create schedule, error:%w", err)
	}

	return nil
}

func RegisterCronWorker(config *config.Config, temporalClient temporal.Client, activities *activity.Activities) {

	wfWorker := temporalClient.NewWorker(CronJobQueueName, worker.Options{})
	wfWorker.RegisterActivity(activities)
	wfWorker.RegisterWorkflow(SyncAsClientWorkflow)
	wfWorker.RegisterWorkflow(CalcRecomScoreWorkflow)

}
