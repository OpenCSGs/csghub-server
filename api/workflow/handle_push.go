package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/types"
)

func HandlePushWorkflow(ctx workflow.Context, req *types.GiteaCallbackPushReq) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("handle push workflow started", "repo_path", req.Repository.FullName)

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	// Watch space change
	err := workflow.ExecuteActivity(actCtx, activities.WatchSpaceChange, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to watch space change", "error", err, "req", req)
		return err
	}

	// Watch repo relation
	err = workflow.ExecuteActivity(actCtx, activities.WatchRepoRelation, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to watch repo relation", "error", err, "req", req)
		return err
	}

	// Set repo update time
	err = workflow.ExecuteActivity(actCtx, activities.SetRepoUpdateTime, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to set repo update time", "error", err, "req", req)
		return err
	}

	// Update repo infos
	err = workflow.ExecuteActivity(actCtx, activities.UpdateRepoInfos, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to update repo infos", "error", err, "req", req)
		return err
	}

	// Sensitive check
	err = workflow.ExecuteActivity(actCtx, activities.SensitiveCheck, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to sensitive check", "error", err, "req", req)
		return err
	}

	logger.Info("handle push workflow ended", "repo_path", req.Repository.FullName)

	return nil
}
