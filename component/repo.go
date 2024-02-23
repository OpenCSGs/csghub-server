package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type repoComponent struct {
	user      *database.UserStore
	org       *database.OrgStore
	namespace *database.NamespaceStore
	repo      *database.RepoStore
	git       gitserver.GitServer
}

func (c *repoComponent) CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, error) {
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		if namespace.UserID != user.ID {
			return nil, nil, errors.New("users do not have permission to create spaces in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, nil, errors.New("users do not have permission to create spaces in this namespace")
		}
	}

	gitRepoReq := gitserver.CreateRepoReq{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Name,
		License:       req.License,
		DefaultBranch: "main",
		// Readme:        "Please introduce your space.",
		Readme:   req.Readme,
		Private:  req.Private,
		RepoType: req.RepoType,
	}
	gitRepo, err := c.git.CreateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to create repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("fail to create repo in git, error: %w", err)
	}

	dbRepo := database.Repository{
		UserID:         user.ID,
		Path:           path.Join(req.Namespace, req.Name),
		GitPath:        gitRepo.GitPath,
		Name:           req.Name,
		Private:        req.Private,
		License:        req.License,
		DefaultBranch:  "main",
		RepositoryType: req.RepoType,
		HTTPCloneURL:   gitRepo.HttpCloneURL,
		SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.CreateRepo(ctx, dbRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to create database repo, error: %w", err)
	}

	return gitRepo, newDBRepo, nil
}

func (c *repoComponent) UpdateRepo(ctx context.Context, req types.CreateRepoReq) (*database.Repository, error) {
	repo, err := c.repo.Find(ctx, req.Namespace, string(req.RepoType), req.Name)
	if err != nil {
		return nil, errors.New("repository does not exist")
	}

	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		if namespace.UserID != user.ID {
			return nil, errors.New("users do not have permission to create spaces in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, errors.New("users do not have permission to create spaces in this namespace")
		}
	}

	gitRepoReq := gitserver.UpdateRepoReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
		Description:   req.Description,
		DefaultBranch: req.DefaultBranch,
		Private:       req.Private,
		RepoType:      req.RepoType,
	}
	gitRepo, err := c.git.UpdateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in git, error: %w", err)
	}

	repo.Name = gitRepo.Nickname
	repo.Description = gitRepo.Description
	repo.Private = gitRepo.Private
	repo.DefaultBranch = gitRepo.DefaultBranch

	resRepo, err := c.repo.UpdateRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in database, error: %w", err)
	}

	return resRepo, nil
}
