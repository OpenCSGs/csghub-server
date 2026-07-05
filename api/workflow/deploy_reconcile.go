package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func DeployReconcileWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("deploy reconcile workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 10,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activities.ReconcileAllStatus).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to reconcile deploy status", "error", err)
		return err
	}
	return nil
}
