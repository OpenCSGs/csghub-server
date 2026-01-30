package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func BatchMigrateToXnetWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("batch migrate to xnet workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 24,
		TaskQueue:           BatchMigrateQueueName,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activities.BatchMigrateToXnet).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to batch migrate to xnet", "error", err)
		return err
	}
	logger.Info("batch migrate to xnet workflow completed")
	return nil
}
