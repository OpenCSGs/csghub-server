package workflow

import (
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/types"
)

// CalculateRepoSizeWorkflow is a workflow that only calculates repo size
func CalculateRepoSizeWorkflow(ctx workflow.Context, req *types.GiteaCallbackPushReq) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("calculate repo size workflow started", slog.Any("req.Repository.FullName", req.Repository.FullName))

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)

	// Calculate repo size
	err := workflow.ExecuteActivity(actCtx, activities.CalculateRepoSize, req).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to calculate repo size", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	logger.Info("calculate repo size workflow ended", slog.Any("req.Repository.FullName", req.Repository.FullName))

	return nil
}
