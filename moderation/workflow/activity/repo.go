package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
)

// GenRepoFileList generates repository file records for a given repository.
// This function is an activity that can be used in a workflow.
func GenRepoFileList(ctx context.Context, repo *database.Repository, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("gen repo files start", "repo_path", repo.Path)
	rfc, err := component.NewRepoFileComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo file component, error: %w", err)
	}
	return rfc.GenRepoFileRecords(ctx, repo)
}

func CheckRepoFiles(ctx context.Context, repo *database.Repository, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("check repo files start", "repo_path", repo.Path)
	rc, err := component.NewRepoComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo component, error: %w", err)
	}
	err = rc.CheckRepoFiles(ctx, repo.ID, component.CheckOption{})
	if err != nil {
		logger.Error("check repo files failed", "error", err, "repo_path", repo.Path)
		return err
	}
	logger.Info("check repo files complete", "repo_path", repo.Path)
	return nil
}

// RepoSensitiveCheckPending updates the sensitive check status of a repository to pending.
// This function is an activity that can be used in a workflow.
func RepoSensitiveCheckPending(ctx context.Context, repo *database.Repository, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("set repo sensitive check status to pending start", "path", repo.Path)
	rc, err := component.NewRepoComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo component, error: %w", err)
	}
	return rc.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPending)
}

func DetectRepoSensitiveCheckStatus(ctx context.Context, repo *database.Repository, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("detect repo sensitive check status start", "repo_path", repo.Path)
	rfc, err := component.NewRepoFileComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo file component, error: %w", err)
	}
	// TODO: handle other branches
	return rfc.DetectRepoSensitiveCheckStatus(ctx, repo.ID, repo.DefaultBranch)
}
