package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activity.DeleteUserAndRelations, user, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to delete user and relations", "error", err, "user", user)
		return err
	}

	return nil
}

func UserSoftDeletionWorkflow(ctx workflow.Context, user common.User, req types.CloseAccountReq, config *config.Config) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("user soft deletion workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activity.SoftDeleteUserAndRelations, user, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to soft delete user and relations", "error", err, "user", user)
		return err
	}

	return nil
}
