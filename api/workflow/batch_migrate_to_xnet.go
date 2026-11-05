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

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		TaskQueue:           BatchMigrateQueueName,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, options)

	for iteration := 0; ; iteration++ {
		var result types.XnetDispatchBatchResult
		if err := workflow.ExecuteActivity(actCtx, activities.BatchMigrateToXnet).Get(ctx, &result); err != nil {
			if temporal.IsCanceledError(err) {
				return err
			}
			logger.Error("failed to dispatch xnet migration batch", "error", err)
		} else if result.Done {
			if result.Exhausted > 0 {
				logger.Warn("xnet migration has permanently failed objects", "exhausted", result.Exhausted)
			} else {
				logger.Info("xnet migration dispatcher idle", "completed", result.Completed)
			}
		}
		if iteration >= 499 {
			return workflow.NewContinueAsNewError(ctx, BatchMigrateToXnetWorkflow)
		}
		pollInterval := time.Duration(result.PollSeconds) * time.Second
		if pollInterval <= 0 {
			pollInterval = 5 * time.Second
		}
		if err := workflow.Sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
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
