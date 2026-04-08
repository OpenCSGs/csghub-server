package workflow

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

type repoFullCheck struct {
	repoStore database.RepoStore
}

func newRepoFullCheck() *repoFullCheck {
	return &repoFullCheck{
		repoStore: database.NewRepoStore(),
	}
}

func newRepoFullCheckWithDB(repoStore database.RepoStore) *repoFullCheck {
	return &repoFullCheck{
		repoStore: repoStore,
	}
}

// RepoFullCheckWorkflow has been moved to common.RepoFullCheckWorkflow for better dependency management
func RepoFullCheckWorkflow(ctx workflow.Context, repo common.Repo, cfg *config.Config) error {
	return newRepoFullCheck().Execute(ctx, repo, cfg)
}

func init() {
	common.RepoFullCheckWorkflow = RepoFullCheckWorkflow
}

func (rfc *repoFullCheck) Execute(ctx workflow.Context, repo common.Repo, cfg *config.Config) error {
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
	actCtx := workflow.WithActivityOptions(ctx, options)
	var err error

	repoStore := rfc.repoStore
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dbRepo, err := repoStore.FindByPath(dbCtx, repo.RepoType, repo.Namespace, repo.Name)
	if err != nil {
		return fmt.Errorf("failed to get repo, error: %w", err)
	}
	err = workflow.ExecuteActivity(actCtx, activity.RepoSensitiveCheckPending, repo, cfg).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to update repo sensitive check status", "error", err, "repo", repo, "status", types.SensitiveCheckPending)
		return err
	}

	// 1. generate repo file list
	err = workflow.ExecuteActivity(actCtx, activity.GenRepoFileList, dbRepo, cfg).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to generate repo file list", "error", err, "repo", repo)
		return err
	}
	// 2. check repo file content
	err = workflow.ExecuteActivity(actCtx, activity.CheckRepoFiles, dbRepo, cfg).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to check repo files", "error", err, "repo", repo)
		return err
	}
	// 3. update repo sensitive check status
	err = workflow.ExecuteActivity(actCtx, activity.DetectRepoSensitiveCheckStatus, dbRepo, cfg).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to detect repo sensitive check status", "error", err, "repo", repo)
		return err
	}

	return nil
}
