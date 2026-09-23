package component

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type PromptVersionComponent interface {
	CreatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.CreatePromptVersionReq) (*types.PromptVersion, error)
	ListPromptVersions(ctx context.Context, req types.PromptVersionReq) ([]types.PromptVersion, error)
	GetPromptVersion(ctx context.Context, req types.PromptVersionReq) (*types.PromptVersionDetail, error)
	UpdatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.UpdatePromptReq) (*types.PromptVersionDetail, error)
}

func (c *promptComponentImpl) CreatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.CreatePromptVersionReq) (*types.PromptVersion, error) {
	filePath, err := normalizePromptVersionFilePath(req.FilePath)
	if err != nil {
		return nil, err
	}
	version, err := normalizePromptVersion(body.Version)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptWithPermission(ctx, req, true)
	if err != nil {
		return nil, err
	}

	ref := prompt.Repository.DefaultBranch
	if ref == "" {
		ref = types.MainBranch
	}
	if sourceVersion := strings.TrimSpace(body.SourceVersion); sourceVersion != "" {
		source, err := c.promptVersionStore.ByPromptIDFilePathAndVersion(ctx, prompt.ID, filePath, sourceVersion)
		if err != nil {
			return nil, err
		}
		ref = source.Hash
	}

	commit, err := c.gitServer.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace, Name: req.Name, Ref: ref, RepoType: types.PromptRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("get prompt version commit: %w", err)
	}
	if commit == nil || commit.ID == "" {
		return nil, fmt.Errorf("prompt version commit is empty")
	}
	if _, err := c.ParseJsonFile(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace, Name: req.Name, Ref: commit.ID, Path: filePath, RepoType: types.PromptRepo,
	}); err != nil {
		return nil, fmt.Errorf("validate prompt version file: %w", err)
	}

	created, err := c.promptVersionStore.Create(ctx, database.PromptVersion{
		PromptID: prompt.ID, FilePath: filePath, Version: version, Hash: commit.ID, Changelog: body.Changelog,
	})
	if err != nil {
		return nil, err
	}
	return promptVersionFromDatabase(created), nil
}

func (c *promptComponentImpl) ListPromptVersions(ctx context.Context, req types.PromptVersionReq) ([]types.PromptVersion, error) {
	filePath, err := normalizePromptVersionFilePath(req.FilePath)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptWithPermission(ctx, req, false)
	if err != nil {
		return nil, err
	}
	versions, err := c.promptVersionStore.ByPromptIDAndFilePath(ctx, prompt.ID, filePath)
	if err != nil {
		return nil, err
	}
	result := make([]types.PromptVersion, 0, len(versions))
	for i := range versions {
		result = append(result, *promptVersionFromDatabase(&versions[i]))
	}
	return result, nil
}

func (c *promptComponentImpl) GetPromptVersion(ctx context.Context, req types.PromptVersionReq) (*types.PromptVersionDetail, error) {
	filePath, version, prompt, record, err := c.resolvePromptVersion(ctx, req, false)
	if err != nil {
		return nil, err
	}
	output, err := c.ParseJsonFile(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace, Name: req.Name, Ref: record.Hash, Path: filePath, RepoType: types.PromptRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("read prompt version %s: %w", version, err)
	}
	permission, err := c.repoComponent.GetUserRepoPermission(ctx, req.CurrentUser, prompt.Repository)
	if err != nil {
		return nil, fmt.Errorf("get prompt permission: %w", err)
	}
	output.CanWrite = permission.CanWrite
	output.CanManage = permission.CanAdmin
	return &types.PromptVersionDetail{PromptVersion: *promptVersionFromDatabase(record), Prompt: *output}, nil
}

func (c *promptComponentImpl) UpdatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.UpdatePromptReq) (*types.PromptVersionDetail, error) {
	filePath, _, prompt, record, err := c.resolvePromptVersion(ctx, req, true)
	if err != nil {
		return nil, err
	}
	ref := prompt.Repository.DefaultBranch
	if ref == "" {
		ref = types.MainBranch
	}
	promptReq := types.PromptReq{Namespace: req.Namespace, Name: req.Name, CurrentUser: req.CurrentUser, Path: filePath}
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("find prompt version user: %w", err)
	}
	if _, err := c.updatePromptFile(ctx, promptReq, body, ref, &user); err != nil {
		return nil, err
	}
	commit, err := c.gitServer.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace, Name: req.Name, Ref: ref, RepoType: types.PromptRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("get saved prompt version commit: %w", err)
	}
	if commit == nil || commit.ID == "" {
		return nil, fmt.Errorf("saved prompt version commit is empty")
	}
	updated, err := c.promptVersionStore.UpdateHash(ctx, record.ID, commit.ID)
	if err != nil {
		return nil, err
	}
	record.Hash = commit.ID
	record.UpdatedAt = updated.UpdatedAt
	output := types.PromptOutput{Prompt: body.Prompt, FilePath: filePath, CanWrite: true}
	return &types.PromptVersionDetail{PromptVersion: *promptVersionFromDatabase(record), Prompt: output}, nil
}

func (c *promptComponentImpl) resolvePromptVersion(ctx context.Context, req types.PromptVersionReq, write bool) (string, string, *database.Prompt, *database.PromptVersion, error) {
	filePath, err := normalizePromptVersionFilePath(req.FilePath)
	if err != nil {
		return "", "", nil, nil, err
	}
	version, err := normalizePromptVersion(req.Version)
	if err != nil {
		return "", "", nil, nil, err
	}
	prompt, err := c.promptWithPermission(ctx, req, write)
	if err != nil {
		return "", "", nil, nil, err
	}
	record, err := c.promptVersionStore.ByPromptIDFilePathAndVersion(ctx, prompt.ID, filePath, version)
	if err != nil {
		return "", "", nil, nil, err
	}
	return filePath, version, prompt, record, nil
}

func (c *promptComponentImpl) promptWithPermission(ctx context.Context, req types.PromptVersionReq, write bool) (*database.Prompt, error) {
	prompt, err := c.promptStore.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	permission, err := c.repoComponent.GetUserRepoPermission(ctx, req.CurrentUser, prompt.Repository)
	if err != nil {
		return nil, fmt.Errorf("get prompt permission: %w", err)
	}
	if write && !permission.CanWrite {
		return nil, errorx.ErrForbidden
	}
	if !write && !permission.CanRead {
		return nil, errorx.ErrForbidden
	}
	return prompt, nil
}

func normalizePromptVersionFilePath(filePath string) (string, error) {
	filePath = strings.TrimSpace(strings.TrimPrefix(filePath, "/"))
	cleaned := path.Clean(filePath)
	if filePath == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(filePath) {
		return "", errorx.ReqParamInvalid(errors.New("invalid prompt file path"), errorx.Ctx().Set("file_path", filePath))
	}
	if !strings.HasSuffix(cleaned, ".jsonl") {
		return "", errorx.ReqParamInvalid(errors.New("prompt file path must end with .jsonl"), errorx.Ctx().Set("file_path", filePath))
	}
	return cleaned, nil
}

func normalizePromptVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", errorx.ReqParamInvalid(errors.New("version is required"), errorx.Ctx().Set("version", version))
	}
	return version, nil
}

func promptVersionFromDatabase(version *database.PromptVersion) *types.PromptVersion {
	return &types.PromptVersion{
		ID: version.ID, Version: version.Version, FilePath: version.FilePath, Commit: version.Hash,
		Changelog: version.Changelog, CreatedAt: version.CreatedAt, UpdatedAt: version.UpdatedAt,
	}
}
