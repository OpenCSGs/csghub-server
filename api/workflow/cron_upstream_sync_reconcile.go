package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// UpstreamSyncReconcileWorkflow is a cron-triggered workflow that performs
// full reconciliation of deploy→upstream sync. It calls the ReconcileUpstreams
// activity which: (1) syncs all running deploys to upstreams, and (2) disables
// ghost upstreams whose deploys are no longer running.
func UpstreamSyncReconcileWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("upstream sync reconcile workflow started")

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 10,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	err := workflow.ExecuteActivity(actCtx, activities.UpstreamSyncReconcile).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to reconcile upstreams", "error", err)
		return err
	}
	logger.Info("upstream sync reconcile workflow finished")
	return nil
}
