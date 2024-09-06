package component

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type MultiSyncComponent struct {
	s            *database.MultiSyncStore
	repo         *database.RepoStore
	model        *database.ModelStore
	dataset      *database.DatasetStore
	namespace    *database.NamespaceStore
	user         *database.UserStore
	versionStore *database.SyncVersionStore
	tag          *database.TagStore
	file         *database.FileStore
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
		tag:          database.NewTagStore(),
		file:         database.NewFileStore(),
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

	syncVersions, err := c.s.GetAfterDistinct(ctx, v.Version)
	if err != nil {
		slog.Error("failed to find distinct sync versions", slog.Any("error", err))
		return err
	}
	for _, v := range syncVersions {
		sv := types.SyncVersion{
			Version:        v.Version,
			SourceID:       v.SourceID,
			RepoPath:       v.RepoPath,
			RepoType:       v.RepoType,
			LastModifyTime: v.LastModifiedAt,
			ChangeLog:      v.ChangeLog,
		}
		switch v.RepoType {
		case types.ModelRepo:
			ctxGetModel, cancel := context.WithTimeout(ctx, 10*time.Second)
			modelInfo, err := sc.ModelInfo(ctxGetModel, sv)
			if err != nil {
				slog.Error("failed to get model info from client", slog.Any("sync_version", v))
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetModel, sv)
			if err != nil {
				slog.Error("failed to get model readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			modelInfo.Readme = ReadMeData
			ctxCreateModel, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalModel(ctxCreateModel, modelInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			}
		case types.DatasetRepo:
			ctxGetDataset, cancel := context.WithTimeout(ctx, 10*time.Second)
			datasetInfo, err := sc.DatasetInfo(ctxGetDataset, sv)
			if err != nil {
				slog.Error("failed to get model info from client", slog.Any("sync_version", v))
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetDataset, sv)
			if err != nil {
				slog.Error("failed to get model readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			datasetInfo.Readme = ReadMeData
			ctxCreateDataset, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalDataset(ctxCreateDataset, datasetInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			}
		default:
			slog.Error("failed to create local synced repo, unsupported repo type", slog.Any("sync_version", v), slog.Any("error", err))
		}
	}

	return nil
}

func (c *MultiSyncComponent) createLocalDataset(ctx context.Context, m *types.Dataset, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
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
			Email:    common.AddPrefixBySourceID(s.SourceID, m.User.Email),
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
		Readme:      m.Readme,
		// License:        req.License,
		// DefaultBranch:  gitRepo.DefaultBranch,
		RepositoryType: types.DatasetRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.UpdateOrCreateRepo(ctx, dbRepo)
	if err != nil {
		return fmt.Errorf("fail to create database repo, error: %w", err)
	}

	if len(m.Tags) > 0 {
		var repoTags []database.RepositoryTag
		for _, tag := range m.Tags {
			dbTag := database.Tag{
				Name:     tag.Name,
				Category: tag.Category,
				Group:    tag.Group,
				BuiltIn:  tag.BuiltIn,
				ShowName: tag.ShowName,
				Scope:    database.DatasetTagScope,
			}
			t, err := c.tag.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}

		err = c.repo.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}

		err = c.repo.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to create database tag", slog.Any("error", err))
		}
	}

	err = c.repo.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil {
		slog.Error("failed to delete database files", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil {
		slog.Error("failed to get all files of repo", slog.Any("sync_version", s), slog.Any("error", err))
	}
	if len(files) > 0 {
		var dbFiles []database.File
		for _, f := range files {
			dbFiles = append(dbFiles, database.File{
				Name:              f.Name,
				Path:              f.Path,
				ParentPath:        common.ConvertDotToSlash(filepath.Dir(f.Path)),
				Size:              f.Size,
				LastCommitMessage: f.Commit.Message,
				LastCommitDate:    f.Commit.CommitterDate,
				RepositoryID:      newDBRepo.ID,
			})
		}

		err = c.file.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new dataset record related to repo
	dbDataset := database.Dataset{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.dataset.CreateIfNotExist(ctx, dbDataset)
	if err != nil {
		return fmt.Errorf("failed to create dataset in db, cause: %w", err)
	}
	return nil

}
func (c *MultiSyncComponent) createLocalModel(ctx context.Context, m *types.Model, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
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
			Email:    common.AddPrefixBySourceID(s.SourceID, m.User.Email),
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
		Readme:      m.Readme,
		// License:        req.License,
		// DefaultBranch:  gitRepo.DefaultBranch,
		RepositoryType: types.ModelRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.UpdateOrCreateRepo(ctx, dbRepo)
	if err != nil {
		return fmt.Errorf("fail to create database repo, error: %w", err)
	}

	if len(m.Tags) > 0 {
		var repoTags []database.RepositoryTag
		for _, tag := range m.Tags {
			dbTag := database.Tag{
				Name:     tag.Name,
				Category: tag.Category,
				Group:    tag.Group,
				BuiltIn:  tag.BuiltIn,
				ShowName: tag.ShowName,
				Scope:    database.ModelTagScope,
			}
			t, err := c.tag.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}
		err = c.repo.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}
		err = c.repo.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to batch create database tag", slog.Any("error", err))
		}
	}

	err = c.repo.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil {
		slog.Error("failed to delete all files for repo", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil {
		slog.Error("failed to get all files of repo", slog.Any("sync_version", s), slog.Any("error", err))
	}
	if len(files) > 0 {
		var dbFiles []database.File
		for _, f := range files {
			dbFiles = append(dbFiles, database.File{
				Name:              f.Name,
				Path:              f.Path,
				ParentPath:        common.ConvertDotToSlash(filepath.Dir(f.Path)),
				Size:              f.Size,
				LastCommitMessage: f.Commit.Message,
				LastCommitDate:    f.Commit.CommitterDate,
				RepositoryID:      newDBRepo.ID,
			})
		}

		err = c.file.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new model record related to repo
	dbModel := database.Model{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
		BaseModel:    m.BaseModel,
	}
	_, err = c.model.CreateIfNotExist(ctx, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}
	return nil
}

func (c *MultiSyncComponent) createUser(ctx context.Context, req types.CreateUserRequest) (database.User, error) {
	gsUserReq := gitserver.CreateUserRequest{
		Nickname: req.Name,
		Username: req.Username,
		Email:    req.Email,
	}
	gsUserResp, err := c.git.CreateUser(gsUserReq)
	if err != nil {
		newError := fmt.Errorf("failed to create gitserver user,error:%w", err)
		return database.User{}, newError
	}

	namespace := &database.Namespace{
		Path:     req.Username,
		Mirrored: true,
	}
	user := &database.User{
		NickName: req.Name,
		Username: req.Username,
		Email:    req.Email,
		GitID:    gsUserResp.GitID,
		Password: gsUserResp.Password,
	}
	err = c.user.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user,error:%w", err)
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
