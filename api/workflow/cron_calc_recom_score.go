package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/common/config"
)

func CalcRecomScoreWorkflow(ctx workflow.Context, config *config.Config) error {
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
	err := workflow.ExecuteActivity(ctx, activity.CalcRecomScore, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to calc recom score", "error", err)
		return err
	}
	return nil
}
