package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow/activity"
)

const WorkflowBroadcastEmailQueueName = "workflow_broadcast_email_queue"

func BroadcastEmailWorkflow(ctx workflow.Context, emailReq types.EmailReq, emailPageSize int) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("broadcast email workflow started", "subject", emailReq.Subject)

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Hour,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)

	// default to get email from user
	getEmailsActivity := GetEmailFromUserActivity
	if emailReq.Source == types.EmailSourceNotificationSetting {
		getEmailsActivity = GetEmailFromNotificationSettingActivity
	}

	page := 1
	morePages := true
	for morePages {
		input := activity.BatchSendEmail{
			Page:     page,
			PageSize: emailPageSize,
			EmailReq: emailReq,
		}
		var batchResult activity.BatchGetEmailResult
		err := workflow.ExecuteActivity(actCtx, getEmailsActivity, input).Get(ctx, &batchResult)
		if err != nil {
			return fmt.Errorf("failed to get email list, error: %w", err)
		}
		morePages = batchResult.MorePages
		page++

		if len(batchResult.Emails) > 0 {
			err := workflow.ExecuteActivity(actCtx, SendEmailBatchActivity, emailReq, batchResult.Emails).Get(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to send email, error: %w", err)
			}
		}
	}
	return nil
}
