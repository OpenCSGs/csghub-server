package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	activity "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow/activity"
)

func BroadcastInternalMessageWorkflow(ctx workflow.Context, messageId string, userPageSize int) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("broadcast message workflow started", "messageId", messageId)

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)

	page := 1
	morePages := true
	for morePages {
		input := activity.BatchInsertUserMessage{
			Page:      page,
			PageSize:  userPageSize,
			MessageId: messageId,
		}
		var batchResult activity.BatchInsertUserMessageResult
		err := workflow.ExecuteActivity(actCtx, "InsertUserMessageBatchActivity", input).Get(ctx, &batchResult)
		if err != nil {
			return fmt.Errorf("failed to insert user messages, error: %w", err)
		}
		morePages = batchResult.MorePages
		page++

		if len(batchResult.FailedUserMessages) > 0 {
			err := workflow.ExecuteActivity(actCtx, "LogUserMessageFailuresActivity", batchResult.FailedUserMessages).Get(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to log user message failures, error: %w", err)
			}
		}
	}
	logger.Info("broadcast message workflow completed", "messageId", messageId)

	return nil
}
