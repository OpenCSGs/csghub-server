package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func SyncAsClientWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("sync as client workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activities.SyncAsClient).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to sync as client", "error", err)
		return err
	}
	logger.Info("sync as client workflow finished")
	return nil
}
