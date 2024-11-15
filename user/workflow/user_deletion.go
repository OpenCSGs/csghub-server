package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/workflow/activity"
	"opencsg.com/csghub-server/user/workflow/common"
)

func UserDeletionWorkflow(ctx workflow.Context, user common.User, config *config.Config) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("user deletion workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(ctx, activity.DeleteUserAndRelations, user, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to delete user and relations", "error", err, "user", user)
		return err
	}

	return nil
}
