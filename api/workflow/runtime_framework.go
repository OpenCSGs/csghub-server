package workflow

import (
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
)

func RuntimeFrameworkWorkflow(ctx workflow.Context, req types.RuntimeFrameworkModels) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("runtime framework workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 2,
		RetryPolicy:         retryPolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	if dl, ok := ctx.Deadline(); ok {
		slog.Info("current time", slog.Any("current_time", time.Now()), slog.Any("deadline", dl))
	}
	err := workflow.ExecuteActivity(ctx, activities.RuntimeFrameworkScan, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to run runtime framework scan activity", "error", err)
		return err
	}
	logger.Info("runtime framework scanning workflow completed")
	return nil
}
