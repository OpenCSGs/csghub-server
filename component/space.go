package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceComponent(config *config.Config) (*SpaceComponent, error) {
	c := &SpaceComponent{}
	c.user = database.NewUserStore()
	c.space = database.NewSpaceStore()
	c.org = database.NewOrgStore()
	c.namespace = database.NewNamespaceStore()
	c.repo = database.NewRepoStore()
	var err error
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type SpaceComponent struct {
	user      *database.UserStore
	space     *database.SpaceStore
	org       *database.OrgStore
	namespace *database.NamespaceStore
	repo      *database.RepoStore
	git       gitserver.GitServer
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Creator)
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

	gitRepoReq := gitserver.CreateRepoReq{
		Username:  req.Creator,
		Namespace: req.Namespace,
		Name:      req.Name,
		License:   req.License,
		Readme:    "",
		Private:   req.Private,
		RepoType:  types.SpaceRepo,
	}
	gitRepo, err := c.git.CreateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to create space in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in git, error: %w", err)
	}

	dbRepo := database.Repository{
		UserID:         user.ID,
		Path:           path.Join(req.Namespace, req.Name),
		GitPath:        gitRepo.GitPath,
		Name:           req.Name,
		Private:        req.Private,
		License:        req.License,
		RepositoryType: types.SpaceRepo,
		HTTPCloneURL:   gitRepo.HttpCloneURL,
		SSHCloneURL:    gitRepo.SshCloneURL,
	}

	dbSpace := database.Space{
		Name:    req.Name,
		UrlSlug: gitRepo.Nickname,
		Path:    path.Join(req.Namespace, req.Name),
		GitPath: gitRepo.GitPath,
		// update after repo is created
		RepositoryID:  0,
		LastUpdatedAt: time.Now(),
		Private:       req.Private,
		UserID:        user.ID,
		Sdk:           req.Sdk,
	}

	// create space and repository in a sql transaction
	tx, err := c.space.BeginTx(ctx)
	if err != nil {
		slog.Error("fail to start tx", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to start tx, error: %w", err)
	}

	newDBRepo, err := c.repo.CreateRepoTx(ctx, tx, dbRepo)
	if err != nil {
		tx.Rollback()
		slog.Error("fail to create repository in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create repository in db, error: %w", err)
	}

	dbSpace.RepositoryID = newDBRepo.ID
	_, err = c.space.CreateTx(ctx, tx, dbSpace)
	if err != nil {
		tx.Rollback()
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
	}

	if err := tx.Commit(); err != nil {
		slog.Error("fail to commit tx", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to commit tx, error: %w", err)
	}

	space := &types.Space{
		Creator:   req.Creator,
		Namespace: req.Namespace,
		Name:      req.Name,
		License:   req.License,
		Sdk:       req.Sdk,
		// TODO: get running status and endpoint from inference service
		Endpoint:      "",
		RunningStatus: "",
		Private:       req.Private,
	}
	return space, nil
}
