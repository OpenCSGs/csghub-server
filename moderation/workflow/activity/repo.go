package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

// GenRepoFileList generates repository file records for a given repository.
// This function is an activity that can be used in a workflow.
func GenRepoFileList(ctx context.Context, req common.Repo, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("gen repo files start", "repo", req)
	rfc, err := component.NewRepoFileComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo file component, error: %w", err)
	}
	return rfc.GenRepoFileRecords(ctx, req.RepoType, req.Namespace, req.Name)
}

func CheckRepoFiles(ctx context.Context, req common.Repo, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("check repo files start", "repoType", req.RepoType, "namespace", req.Namespace, "name", req.Name)
	rc, err := component.NewRepoComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo component, error: %w", err)
	}

	err = rc.CheckRepoFiles(ctx, req.RepoType, req.Namespace, req.Name, component.CheckOption{})
	if err != nil {
		logger.Error("check repo files failed", "error", err, "repoType", req.RepoType, "namespace", req.Namespace, "name", req.Name)
		return err
	}
	logger.Info("check repo files complete", "repoType", req.RepoType, "namespace", req.Namespace, "name", req.Name)
	return nil
}

// RepoSensitiveCheckPending updates the sensitive check status of a repository to pending.
// This function is an activity that can be used in a workflow.
func RepoSensitiveCheckPending(ctx context.Context, req common.Repo, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("set repo sensitive check status to pending start", "repoType", req.RepoType, "namespace", req.Namespace, "name", req.Name)
	rc, err := component.NewRepoComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo component, error: %w", err)
	}

	return rc.UpdateRepoSensitiveCheckStatus(ctx, req.RepoType, req.Namespace, req.Name, types.SensitiveCheckPending)
}

func DetectRepoSensitiveCheckStatus(ctx context.Context, req common.Repo, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("detect repo sensitive check status start", "repoType", req.RepoType, "namespace", req.Namespace, "name", req.Name)
	rfc, err := component.NewRepoFileComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create repo component, error: %w", err)
	}

	return rfc.DetectRepoSensitiveCheckStatus(ctx, req.RepoType, req.Namespace, req.Name)
}
