package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func CalcRecomScoreWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("calc recom score workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(ctx, activities.CalcRecomScore).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to calc recom score", "error", err)
		return err
	}
	return nil
}
