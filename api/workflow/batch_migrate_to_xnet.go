package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
)

// BatchMigrateToXnetWorkflow preserves the existing all-repository activity command.
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

// BatchMigrateModelsToXnetWorkflow migrates only models without changing existing workflow histories.
func BatchMigrateModelsToXnetWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("batch model migration to xnet workflow started")

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 24,
		TaskQueue:           BatchMigrateQueueName,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	actCtx := workflow.WithActivityOptions(ctx, options)
	if err := workflow.ExecuteActivity(actCtx, activities.BatchMigrateToXnetForType, types.ModelRepo).Get(ctx, nil); err != nil {
		logger.Error("failed to batch migrate models to xnet", "error", err)
		return err
	}
	logger.Info("batch model migration to xnet workflow completed")
	return nil
}
