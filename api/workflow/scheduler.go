package workflow

import (
	"context"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/types"
)

const (
	CronJobQueueName      = "workflow_cron_queue"
	BatchMigrateQueueName = "workflow_batch_migrate_queue"
)

// createSchedule creates a Temporal cron schedule with SKIP overlap policy.
// If the schedule already exists (AlreadyScheduledMessage), the error is
// treated as success. This helper is shared across CE/EE/SaaS variants to
// avoid duplicating the boilerplate scheduler.Create + error-check code.
func createSchedule(scheduler temporal.ScheduleClient, id, cronExpr, workflowID string, workflow interface{}) error {
	_, err := scheduler.Create(context.Background(), client.ScheduleOptions{
		ID: id,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{cronExpr},
		},
		Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			TaskQueue: CronJobQueueName,
			Workflow:  workflow,
			Args:      []interface{}{},
		},
	})
	if err != nil && err.Error() != types.AlreadyScheduledMessage {
		return fmt.Errorf("unable to create %s schedule, error:%w", id, err)
	}
	return nil
}
