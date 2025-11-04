package workflow

import (
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/types"
)

func HandlePushWorkflow(ctx workflow.Context, req *types.GiteaCallbackPushReq) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("[git_callback] handle push workflow started", slog.Any("req.Repository.FullName", req.Repository.FullName))

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 1,
		RetryPolicy:         retryPolicy,
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	// Watch space change
	err := workflow.ExecuteActivity(actCtx, activities.WatchSpaceChange, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to watch space change", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Watch repo relation
	err = workflow.ExecuteActivity(actCtx, activities.WatchRepoRelation, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to watch repo relation", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Generate sync versions
	err = workflow.ExecuteActivity(actCtx, activities.GenSyncVersion, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to generate sync versions", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Set repo update time
	err = workflow.ExecuteActivity(actCtx, activities.SetRepoUpdateTime, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to set repo update time", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Sensitive check
	err = workflow.ExecuteActivity(actCtx, activities.SensitiveCheck, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to sensitive check", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Model tree ingestion
	err = workflow.ExecuteActivity(actCtx, activities.UpdateModelTree, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to update model tree", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// MCP scan
	err = workflow.ExecuteActivity(actCtx, activities.MCPScan, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to do mcp scan", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	// Update repo infos
	err = workflow.ExecuteActivity(actCtx, activities.UpdateRepoInfos, req).Get(ctx, nil)
	if err != nil {
		logger.Error("[git_callback] failed to update repo infos", slog.Any("error", err), slog.Any("req", req))
		return err
	}

	logger.Info("[git_callback] handle push workflow ended", slog.Any("req.Repository.FullName", req.Repository.FullName))

	return nil
}
