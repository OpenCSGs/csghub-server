package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

func RepoFullCheckWorkflow(ctx workflow.Context, repo common.Repo, config *config.Config) error {
	logger := workflow.GetLogger(ctx)

	retryPolicy := &temporal.RetryPolicy{
		// InitialInterval:    time.Second,
		// BackoffCoefficient: 2.0,
		// MaximumInterval:    time.Minute,
		MaximumAttempts: 3,
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 24 * time.Hour,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	var err error

	err = workflow.ExecuteActivity(ctx, activity.RepoSensitiveCheckPending, repo, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to update repo sensitive check status", "error", err, "repo", repo, "status", types.SensitiveCheckPending)
		return err
	}

	// 1. generate repo file list
	err = workflow.ExecuteActivity(ctx, activity.GenRepoFileList, repo, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to generate repo file list", "error", err, "repo", repo)
		return err
	}
	// 2. check repo file content
	err = workflow.ExecuteActivity(ctx, activity.CheckRepoFiles, repo, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to check repo files", "error", err, "repo", repo)
		return err
	}
	// 3. update repo sensitive check status
	err = workflow.ExecuteActivity(ctx, activity.DetectRepoSensitiveCheckStatus, repo, config).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to detect repo sensitive check status", "error", err, "repo", repo)
		return err
	}

	return nil
}
