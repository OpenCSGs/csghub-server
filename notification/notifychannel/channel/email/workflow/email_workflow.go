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
	if len(emailReq.To) > 0 {
		return sendExplicitEmailList(actCtx, emailReq)
	}
	return pageBroadcastEmails(actCtx, emailReq, emailPageSize)
}

func sendExplicitEmailList(ctx workflow.Context, emailReq types.EmailReq) error {
	err := workflow.ExecuteActivity(ctx, SendExplicitEmailListActivity, emailReq).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to send email, error: %w", err)
	}
	return nil
}

func pageBroadcastEmails(ctx workflow.Context, emailReq types.EmailReq, emailPageSize int) error {
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
		err := workflow.ExecuteActivity(ctx, getEmailsActivity, input).Get(ctx, &batchResult)
		if err != nil {
			return fmt.Errorf("failed to get email list, error: %w", err)
		}
		morePages = batchResult.MorePages
		page++

		if err := sendEmailBatch(ctx, emailReq, batchResult.Emails); err != nil {
			return err
		}
	}
	return nil
}

func sendEmailBatch(ctx workflow.Context, emailReq types.EmailReq, emails []string) error {
	if len(emails) == 0 {
		return nil
	}
	err := workflow.ExecuteActivity(ctx, SendEmailBatchActivity, emailReq, emails).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to send email, error: %w", err)
	}
	return nil
}
