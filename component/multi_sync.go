package component

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MultiSyncComponent struct {
	s            *database.MultiSyncStore
	repo         *database.RepoStore
	model        *database.ModelStore
	dataset      *database.DatasetStore
	namespace    *database.NamespaceStore
	user         *database.UserStore
	versionStore *database.SyncVersionStore
	git          gitserver.GitServer
}

func NewMultiSyncComponent(config *config.Config) (*MultiSyncComponent, error) {
	git, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server: %w", err)
	}
	return &MultiSyncComponent{
		s:            database.NewMultiSyncStore(),
		repo:         database.NewRepoStore(),
		model:        database.NewModelStore(),
		dataset:      database.NewDatasetStore(),
		namespace:    database.NewNamespaceStore(),
		user:         database.NewUserStore(),
		versionStore: database.NewSyncVersionStore(),
		git:          git,
	}, nil
}

func (c *MultiSyncComponent) More(ctx context.Context, cur int64, limit int64) ([]types.SyncVersion, error) {
	dbVersions, err := c.s.GetAfter(ctx, cur, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync versions after %d from db: %w", cur, err)
	}
	var versions []types.SyncVersion
	for _, v := range dbVersions {
		versions = append(versions, types.SyncVersion{
			Version:        v.Version,
			SourceID:       v.SourceID,
			RepoPath:       v.RepoPath,
			RepoType:       v.RepoType,
			LastModifyTime: v.LastModifiedAt,
			ChangeLog:      v.ChangeLog,
		})
	}
	return versions, nil
}

func (c *MultiSyncComponent) SyncAsClient(ctx context.Context, sc multisync.Client) error {
	var currentVersion int64
	v, err := c.s.GetLatest(ctx)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to get latest sync version from db: %w", err)
		} else {
			currentVersion = 0
		}
	}

	currentVersion = v.Version
	var hasMore = true
	for hasMore {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := sc.Latest(ctxWithTimeout, currentVersion)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to sync latest version from client, current version:%d, error: %w", currentVersion, err)
		}
		//create local repo
		for _, v := range resp.Data.Versions {
			switch v.RepoType {
			case types.ModelRepo:
				ctxGetModel, cancel := context.WithTimeout(ctx, 10*time.Second)
				modelInfo, err := sc.ModelInfo(ctxGetModel, v)
				cancel()
				if err != nil {
					slog.Error("failed to get model info from client", slog.Any("sync_version", v))
					continue
				}
				ctxCreateModel, cancel := context.WithTimeout(ctx, 5*time.Second)
				err = c.createLocalModel(ctxCreateModel, modelInfo)
				cancel()
				if err != nil {
					slog.Error("failed to create local synced repo", slog.Any("sync_version", v))
				}
			case types.DatasetRepo:
				ctxGetDataset, cancel := context.WithTimeout(ctx, 10*time.Second)
				datasetInfo, err := sc.DatasetInfo(ctxGetDataset, v)
				cancel()
				if err != nil {
					slog.Error("failed to get model info from client", slog.Any("sync_version", v))
					continue
				}
				ctxCreateDataset, cancel := context.WithTimeout(ctx, 5*time.Second)
				err = c.createLocalDataset(ctxCreateDataset, datasetInfo)
				cancel()
				if err != nil {
					slog.Error("failed to create local synced repo", slog.Any("sync_version", v))
				}
			default:
				slog.Error("failed to create local synced repo, unsupported repo type", slog.Any("sync_version", v))
			}
			err := c.createLocalSyncVersion(ctx, v)
			if err != nil {
				slog.Error("failed to create database sync version", slog.Any("sync_version", v), slog.Any("error", err))
				continue
			}
		}
		hasMore = resp.Data.HasMore
		if len(resp.Data.Versions) > 0 {
			currentVersion = resp.Data.Versions[len(resp.Data.Versions)-1].Version
		}
	}

	return nil
}

func (c *MultiSyncComponent) createLocalDataset(ctx context.Context, m *types.Dataset) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = fmt.Sprintf("%s%s", types.OpenCSGPrefix, namespace)
	exists, err := c.repo.Exists(ctx, types.DatasetRepo, namespace, name)
	if err != nil {
		return fmt.Errorf("fail to check if model exists, path:%s/%s, error: %w", namespace, name, err)
	}
	//skip creation
	if exists {
		return nil
	}

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err = c.getUser(ctx, userName)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			Email:    fmt.Sprintf("%s%s", types.OpenCSGPrefix, m.User.Email),
		})
		if err != nil {
			return fmt.Errorf("fail to create user for namespace, namespace:%s, error: %w", namespace, err)
		}
	}
	//create new database repo
	dbRepo := database.Repository{
		UserID: user.ID,
		//new path with prefixed namespace
		Path:        path.Join(namespace, name),
		GitPath:     fmt.Sprintf("%ss_%s/%s", types.DatasetRepo, namespace, name),
		Name:        name,
		Nickname:    m.Nickname,
		Description: m.Description,
		Private:     m.Private,
		// License:        req.License,
		// DefaultBranch:  gitRepo.DefaultBranch,
		RepositoryType: types.ModelRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.CreateRepo(ctx, dbRepo)
	if err != nil {
		return fmt.Errorf("fail to create database repo, error: %w", err)
	}

	//create new dataset record related to repo
	dbDataset := database.Dataset{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.dataset.Create(ctx, dbDataset)
	if err != nil {
		return fmt.Errorf("failed to create dataset in db, cause: %w", err)
	}
	return nil

}
func (c *MultiSyncComponent) createLocalModel(ctx context.Context, m *types.Model) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = fmt.Sprintf("%s%s", types.OpenCSGPrefix, namespace)
	exists, err := c.repo.Exists(ctx, types.ModelRepo, namespace, name)
	if err != nil {
		return fmt.Errorf("fail to check if model exists, path:%s/%s, error: %w", namespace, name, err)
	}
	//skip creation
	if exists {
		return nil
	}

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err = c.getUser(ctx, userName)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			Email:    fmt.Sprintf("%s%s", types.OpenCSGPrefix, m.User.Email),
		})
		if err != nil {
			return fmt.Errorf("fail to create user for namespace, namespace:%s, error: %w", namespace, err)
		}
	}
	//create new database repo
	dbRepo := database.Repository{
		UserID: user.ID,
		//new path with prefixed namespace
		Path:        path.Join(namespace, name),
		GitPath:     fmt.Sprintf("%ss_%s/%s", types.ModelRepo, namespace, name),
		Name:        name,
		Nickname:    m.Nickname,
		Description: m.Description,
		Private:     m.Private,
		// License:        req.License,
		// DefaultBranch:  gitRepo.DefaultBranch,
		RepositoryType: types.ModelRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.CreateRepo(ctx, dbRepo)
	if err != nil {
		return fmt.Errorf("fail to create database repo, error: %w", err)
	}

	//create new model record related to repo
	dbModel := database.Model{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.model.Create(ctx, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}
	return nil
}

func (c *MultiSyncComponent) createUser(ctx context.Context, req types.CreateUserRequest) (database.User, error) {
	user, err := c.git.CreateUser(&req)
	if err != nil {
		newError := fmt.Errorf("failed to create gitserver user,error:%w", err)
		slog.Error(newError.Error())
		return database.User{}, newError
	}

	namespace := &database.Namespace{
		Path:     user.Username,
		Mirrored: true,
	}
	err = c.user.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user,error:%w", err)
		slog.Error(newError.Error())
		return database.User{}, newError
	}

	return *user, err
}

func (c *MultiSyncComponent) getUser(ctx context.Context, userName string) (database.User, error) {
	return c.user.FindByUsername(ctx, userName)
}

func (c *MultiSyncComponent) createLocalSyncVersion(ctx context.Context, v types.SyncVersion) error {
	syncVersion := database.SyncVersion{
		Version:        v.Version,
		SourceID:       v.SourceID,
		RepoPath:       v.RepoPath,
		RepoType:       v.RepoType,
		LastModifiedAt: v.LastModifyTime,
		ChangeLog:      v.ChangeLog,
	}
	err := c.versionStore.Create(ctx, &syncVersion)
	if err != nil {
		return err
	}
	return nil
}
