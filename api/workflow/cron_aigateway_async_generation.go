package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
)

func ProcessAIGatewayAsyncGenerationsWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("aigateway async generation workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}
	listOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retryPolicy,
	}
	processOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	var targets []types.AIGatewayAsyncGenerationTarget
	if err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, listOptions), activities.ListPendingAIGatewayAsyncGenerations).Get(ctx, &targets); err != nil {
		logger.Error("failed to list aigateway async generations", "error", err)
		return err
	}

	if len(targets) > 0 {
		if err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, processOptions), activities.InspectAndMeterAIGatewayAsyncGenerations, targets).Get(ctx, nil); err != nil {
			logger.Warn("unexpected error from aigateway async generation batch", "error", err)
		}
	}

	logger.Info("aigateway async generation workflow completed", "count", len(targets))
	return nil
}
