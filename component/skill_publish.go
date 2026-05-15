package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type SkillPublishComponent interface {
	Publish(ctx context.Context, req *types.PublishSkillVersionReq) (*types.PublishSkillVersionResp, error)
}

func NewSkillPublishComponent(config *config.Config) (SkillPublishComponent, error) {
	repoComponent, err := NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	gitServer, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}

	return &skillPublishComponentImpl{
		repoComponent:     repoComponent,
		gitServer:         gitServer,
		skillStore:        database.NewSkillStore(),
		skillVersionStore: database.NewSkillVersionStore(),
	}, nil
}

type skillPublishComponentImpl struct {
	repoComponent     RepoComponent
	gitServer         gitserver.GitServer
	skillStore        database.SkillStore
	skillVersionStore database.SkillVersionStore
}

func (c *skillPublishComponentImpl) Publish(ctx context.Context, req *types.PublishSkillVersionReq) (*types.PublishSkillVersionResp, error) {
	skill, err := c.skillStore.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, errorx.SkillNotFound(err, errorx.Ctx().Set("namespace", req.Namespace).Set("name", req.Name))
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, req.Username, skill.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission: %w", err)
	}
	if !permission.CanWrite {
		return nil, errorx.ErrForbidden
	}

	version, err := resolvePublishVersion(req.Version)
	if err != nil {
		return nil, err
	}

	ref := skill.Repository.DefaultBranch
	if ref == "" {
		ref = types.MainBranch
	}
	commit, err := c.gitServer.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       ref,
		RepoType:  types.SkillRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest skill commit: %w", err)
	}
	if commit == nil || commit.ID == "" {
		return nil, fmt.Errorf("latest skill commit is empty")
	}

	skillVersion, err := c.createSkillVersion(ctx, skill.ID, version, commit.ID, req)
	if err != nil {
		return nil, err
	}

	return &types.PublishSkillVersionResp{
		Ok:        true,
		SkillID:   fmt.Sprintf("%d", skill.ID),
		VersionID: fmt.Sprintf("%d", skillVersion.ID),
		Version:   skillVersion.Version,
		Commit:    skillVersion.Hash,
	}, nil
}

func (c *skillPublishComponentImpl) createSkillVersion(ctx context.Context, skillID int64, version string, commitHash string, req *types.PublishSkillVersionReq) (*database.SkillVersion, error) {
	existingVersion, err := c.skillVersionStore.BySkillIDAndVersion(ctx, skillID, version)
	if err == nil && existingVersion != nil {
		return nil, errorx.ErrDatabaseDuplicateKey
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.SkillVersionCreateFailed(err, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
	}

	skillVersion, err := c.skillVersionStore.Create(ctx, database.SkillVersion{
		SkillID:   skillID,
		Version:   version,
		Hash:      commitHash,
		Changelog: req.Changelog,
		License:   req.License,
	})
	if err != nil {
		return nil, errorx.SkillVersionCreateFailed(err, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
	}
	return skillVersion, nil
}

func resolvePublishVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version != "" {
		return version, nil
	}
	return "", errorx.ReqParamInvalid(fmt.Errorf("version is required"), errorx.Ctx().Set("param", "version"))
}
