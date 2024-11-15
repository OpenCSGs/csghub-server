package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func HandlePushWorkflow(ctx workflow.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("handle push workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	// Watch space change
	err := workflow.ExecuteActivity(ctx, activity.WatchSpaceChange, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to watch space change", "error", err, "req", req)
		return err
	}

	// Watch repo relation
	err = workflow.ExecuteActivity(ctx, activity.WatchRepoRelation, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to watch repo relation", "error", err, "req", req)
		return err
	}

	// Set repo update time
	err = workflow.ExecuteActivity(ctx, activity.SetRepoUpdateTime, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to set repo update time", "error", err, "req", req)
		return err
	}

	// Update repo infos
	err = workflow.ExecuteActivity(ctx, activity.UpdateRepoInfos, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to update repo infos", "error", err, "req", req)
		return err
	}

	// Sensitive check
	err = workflow.ExecuteActivity(ctx, activity.SensitiveCheck, req, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to sensitive check", "error", err, "req", req)
		return err
	}

	return nil
}
