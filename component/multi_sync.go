package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type multiSyncComponentImpl struct {
	multiSyncStore   database.MultiSyncStore
	repoStore        database.RepoStore
	modelStore       database.ModelStore
	datasetStore     database.DatasetStore
	codeStore        database.CodeStore
	promptStore      database.PromptStore
	mcpStore         database.MCPServerStore
	namespaceStore   database.NamespaceStore
	userStore        database.UserStore
	recomStore       database.RecomStore
	syncVersionStore database.SyncVersionStore
	tagStore         database.TagStore
	fileStore        database.FileStore
	gitServer        gitserver.GitServer
	config           *config.Config
}

type MultiSyncComponent interface {
	More(ctx context.Context, cur int64, limit int64) ([]types.SyncVersion, error)
	SyncAsClient(ctx context.Context, sc multisync.Client) error
}

func NewMultiSyncComponent(config *config.Config) (MultiSyncComponent, error) {
	git, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server: %w", err)
	}
	return &multiSyncComponentImpl{
		multiSyncStore:   database.NewMultiSyncStore(),
		repoStore:        database.NewRepoStore(),
		modelStore:       database.NewModelStore(),
		datasetStore:     database.NewDatasetStore(),
		namespaceStore:   database.NewNamespaceStore(),
		userStore:        database.NewUserStore(),
		recomStore:       database.NewRecomStore(),
		syncVersionStore: database.NewSyncVersionStore(),
		tagStore:         database.NewTagStore(),
		fileStore:        database.NewFileStore(),
		codeStore:        database.NewCodeStore(),
		promptStore:      database.NewPromptStore(),
		mcpStore:         database.NewMCPServerStore(),
		gitServer:        git,
		config:           config,
	}, nil
}

func (c *multiSyncComponentImpl) More(ctx context.Context, cur int64, limit int64) ([]types.SyncVersion, error) {
	dbVersions, err := c.multiSyncStore.GetAfter(ctx, cur, limit)
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

func (c *multiSyncComponentImpl) SyncAsClient(ctx context.Context, sc multisync.Client) error {
	if !c.config.MultiSync.Enabled {
		return nil
	}

	var currentVersion int64
	v, err := c.multiSyncStore.GetLatest(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get latest sync version from db: %w", err)
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

	syncVersions, err := c.multiSyncStore.GetNotCompletedDistinct(ctx)
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
		success := false
		switch v.RepoType {
		case types.ModelRepo:
			ctxGetModel, cancel := context.WithTimeout(ctx, 10*time.Second)
			modelInfo, err := sc.ModelInfo(ctxGetModel, sv)
			if err != nil {
				cancel()
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
			} else {
				success = true
			}
		case types.DatasetRepo:
			ctxGetDataset, cancel := context.WithTimeout(ctx, 10*time.Second)
			datasetInfo, err := sc.DatasetInfo(ctxGetDataset, sv)
			if err != nil {
				cancel()
				slog.Error("failed to get dataset info from client", slog.Any("sync_version", v))
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetDataset, sv)
			if err != nil {
				slog.Error("failed to get dataset readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			datasetInfo.Readme = ReadMeData
			ctxCreateDataset, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalDataset(ctxCreateDataset, datasetInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			} else {
				success = true
			}

		case types.CodeRepo:
			ctxGetCode, cancel := context.WithTimeout(ctx, 10*time.Second)
			codeInfo, err := sc.CodeInfo(ctxGetCode, sv)
			if err != nil {
				slog.Error("failed to get code info from client", slog.Any("sync_version", v))
				cancel()
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetCode, sv)
			if err != nil {
				slog.Error("failed to get code readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			codeInfo.Readme = ReadMeData
			ctxCreateCode, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalCode(ctxCreateCode, codeInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			} else {
				success = true
			}

		case types.MCPServerRepo:
			ctxGetMCPServer, cancel := context.WithTimeout(ctx, 10*time.Second)
			mcpServerInfo, err := sc.MCPServerInfo(ctxGetMCPServer, sv)
			if err != nil {
				slog.Error("failed to get mcpServer info from client", slog.Any("sync_version", v), slog.Any("error", err))
				cancel()
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetMCPServer, sv)
			if err != nil {
				slog.Error("failed to get mcpServer readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			mcpServerInfo.Readme = ReadMeData
			ctxCreateMCPServer, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalMCPServer(ctxCreateMCPServer, mcpServerInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			} else {
				success = true
			}

		case types.PromptRepo:
			ctxGetPrompt, cancel := context.WithTimeout(ctx, 10*time.Second)
			promptInfo, err := sc.PromptInfo(ctxGetPrompt, sv)
			if err != nil {
				slog.Error("failed to get prompt info from client", slog.Any("sync_version", v), slog.Any(("error"), err))
				cancel()
				continue
			}
			ReadMeData, err := sc.ReadMeData(ctxGetPrompt, sv)
			if err != nil {
				slog.Error("failed to get prompt readme from client", slog.Any("sync_version", v), slog.Any("error", err))
			}
			cancel()
			promptInfo.Readme = ReadMeData
			ctxCreatePrompt, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.createLocalPrompt(ctxCreatePrompt, promptInfo, sv, sc)
			cancel()
			if err != nil {
				slog.Error("failed to create local synced repo", slog.Any("sync_version", v), slog.Any("error", err))
			} else {
				success = true
			}

		default:
			slog.Error("failed to create local synced repo, unsupported repo type", slog.Any("sync_version", v), slog.Any("error", err))
		}

		if success {
			ctxCompleteSyncVersion, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = c.syncVersionStore.Complete(ctxCompleteSyncVersion, v)
			cancel()
			if err != nil {
				slog.Error("failed to mark sync version as completed", slog.Any("err", err), slog.Any("sync_version", v))
				// ignore error and continue to next sync version
			}
		}
	}

	return nil
}

func (c *multiSyncComponentImpl) createLocalDataset(ctx context.Context, m *types.Dataset, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			Email:    common.MD5Hash(fmt.Sprintf("%s_%s", userName, m.User.Email)),
			UUID:     uuid.New().String(),
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
		DefaultBranch:  m.DefaultBranch,
		RepositoryType: types.DatasetRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
		MSPath:  m.MultiSource.MSPath,
		HFPath:  m.MultiSource.HFPath,
		CSGPath: m.MultiSource.CSGPath,
	}
	newDBRepo, err := c.repoStore.UpdateOrCreateRepo(ctx, dbRepo)
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
				Scope:    types.DatasetTagScope,
			}
			t, err := c.tagStore.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}

		err = c.repoStore.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}

		err = c.repoStore.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to create database tag", slog.Any("error", err))
		}
	}

	err = c.repoStore.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to delete database files", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

		err = c.fileStore.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new dataset record related to repo
	dbDataset := database.Dataset{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.datasetStore.CreateIfNotExist(ctx, dbDataset)
	if err != nil {
		return fmt.Errorf("failed to create dataset in db, cause: %w", err)
	}

	// create new trending scores related to repo
	if len(m.Scores) != 0 {
		err = c.createLocalRecom(ctx, newDBRepo.ID, m.Scores)
		if err != nil {
			return fmt.Errorf("failed to create database.recom_repo_scores, cause: %w", err)
		}
	}
	return nil

}
func (c *multiSyncComponentImpl) createLocalModel(ctx context.Context, m *types.Model, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			// Add userName to email to avoid email conflict
			Email: common.MD5Hash(fmt.Sprintf("%s_%s", userName, m.User.Email)),
			UUID:  uuid.New().String(),
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
		DefaultBranch:  m.DefaultBranch,
		RepositoryType: types.ModelRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
		MSPath:  m.MultiSource.MSPath,
		HFPath:  m.MultiSource.HFPath,
		CSGPath: m.MultiSource.CSGPath,
	}
	newDBRepo, err := c.repoStore.UpdateOrCreateRepo(ctx, dbRepo)
	if err != nil {
		return fmt.Errorf("fail to create or update database repo, error: %w", err)
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
				Scope:    types.ModelTagScope,
			}
			t, err := c.tagStore.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}
		err = c.repoStore.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}
		err = c.repoStore.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to batch create database tag", slog.Any("error", err))
		}
	}

	err = c.repoStore.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to delete all files for repo", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

		err = c.fileStore.BatchCreate(ctx, dbFiles)
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
	_, err = c.modelStore.CreateIfNotExist(ctx, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}

	// create new trending scores related to repo
	if len(m.Scores) != 0 {
		err = c.createLocalRecom(ctx, newDBRepo.ID, m.Scores)
		if err != nil {
			return fmt.Errorf("failed to create database.recom_repo_scores, cause: %w", err)
		}
	}

	return nil
}

func (c *multiSyncComponentImpl) createLocalCode(ctx context.Context, m *types.Code, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			// Add userName to email to avoid email conflict
			Email: common.MD5Hash(fmt.Sprintf("%s_%s", userName, m.User.Email)),
			UUID:  uuid.New().String(),
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
		GitPath:     fmt.Sprintf("%ss_%s/%s", types.CodeRepo, namespace, name),
		Name:        name,
		Nickname:    m.Nickname,
		Description: m.Description,
		Private:     m.Private,
		Readme:      m.Readme,
		// License:        req.License,
		DefaultBranch:  m.DefaultBranch,
		RepositoryType: types.CodeRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
		MSPath:  m.MultiSource.MSPath,
		HFPath:  m.MultiSource.HFPath,
		CSGPath: m.MultiSource.CSGPath,
	}
	newDBRepo, err := c.repoStore.UpdateOrCreateRepo(ctx, dbRepo)
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
				Scope:    types.CodeTagScope,
			}
			t, err := c.tagStore.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}
		err = c.repoStore.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}
		err = c.repoStore.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to batch create database tag", slog.Any("error", err))
		}
	}

	err = c.repoStore.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to delete all files for repo", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

		err = c.fileStore.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new model record related to repo
	dbCode := database.Code{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.codeStore.CreateIfNotExist(ctx, dbCode)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}

	// create new trending scores related to repo
	if len(m.Scores) != 0 {
		err = c.createLocalRecom(ctx, newDBRepo.ID, m.Scores)
		if err != nil {
			return fmt.Errorf("failed to create database.recom_repo_scores, cause: %w", err)
		}
	}
	return nil
}

func (c *multiSyncComponentImpl) createLocalPrompt(ctx context.Context, m *types.PromptRes, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			// Add userName to email to avoid email conflict
			Email: common.MD5Hash(fmt.Sprintf("%s_%s", userName, m.User.Email)),
			UUID:  uuid.New().String(),
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
		GitPath:     fmt.Sprintf("%ss_%s/%s", types.PromptRepo, namespace, name),
		Name:        name,
		Nickname:    m.Nickname,
		Description: m.Description,
		Private:     m.Private,
		Readme:      m.Readme,
		// License:        req.License,
		DefaultBranch:  m.DefaultBranch,
		RepositoryType: types.PromptRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
		MSPath:  m.MultiSource.MSPath,
		HFPath:  m.MultiSource.HFPath,
		CSGPath: m.MultiSource.CSGPath,
	}
	newDBRepo, err := c.repoStore.UpdateOrCreateRepo(ctx, dbRepo)
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
				Scope:    types.CodeTagScope,
			}
			t, err := c.tagStore.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}
		err = c.repoStore.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}
		err = c.repoStore.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to batch create database tag", slog.Any("error", err))
		}
	}

	err = c.repoStore.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to delete all files for repo", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

		err = c.fileStore.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new model record related to repo
	dbPrompt := database.Prompt{
		Repository:   newDBRepo,
		RepositoryID: newDBRepo.ID,
	}
	_, err = c.promptStore.CreateIfNotExist(ctx, dbPrompt)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}

	// create new trending scores related to repo
	if len(m.Scores) != 0 {
		err = c.createLocalRecom(ctx, newDBRepo.ID, m.Scores)
		if err != nil {
			return fmt.Errorf("failed to create database.recom_repo_scores, cause: %w", err)
		}
	}
	return nil
}

func (c *multiSyncComponentImpl) createLocalMCPServer(ctx context.Context, m *types.MCPServer, s types.SyncVersion, sc multisync.Client) error {
	namespace, name, _ := strings.Cut(m.Path, "/")
	//add prefix to avoid namespace conflict
	namespace = common.AddPrefixBySourceID(s.SourceID, namespace)

	//use namespace as the user login name
	userName := namespace
	var user database.User
	user, err := c.getUser(ctx, userName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail to get user, userName:%s, error: %w", userName, err)
		}
	}
	//user not exists, create new one
	if user.ID == 0 {
		//create as user instead of org, no matter if the namespace is org or user
		user, err = c.createUser(ctx, types.CreateUserRequest{
			Name:     m.User.Nickname,
			Username: userName,
			// Add userName to email to avoid email conflict
			Email: common.MD5Hash(fmt.Sprintf("%s_%s", userName, m.User.Email)),
			UUID:  uuid.New().String(),
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
		GitPath:     fmt.Sprintf("%ss_%s/%s", types.MCPServerRepo, namespace, name),
		Name:        name,
		Nickname:    m.Nickname,
		Description: m.Description,
		Private:     m.Private,
		Readme:      m.Readme,
		// License:        req.License,
		DefaultBranch:  m.DefaultBranch,
		RepositoryType: types.MCPServerRepo,
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		// HTTPCloneURL:   gitRepo.HttpCloneURL,
		// SSHCloneURL:    gitRepo.SshCloneURL,
		MSPath:  m.MultiSource.MSPath,
		HFPath:  m.MultiSource.HFPath,
		CSGPath: m.MultiSource.CSGPath,
	}
	newDBRepo, err := c.repoStore.UpdateOrCreateRepo(ctx, dbRepo)
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
				Scope:    types.CodeTagScope,
			}
			t, err := c.tagStore.FindOrCreate(ctx, dbTag)
			if err != nil {
				slog.Error("failed to create or find database tag", slog.Any("tag", dbTag))
				continue
			}
			repoTags = append(repoTags, database.RepositoryTag{
				RepositoryID: newDBRepo.ID,
				TagID:        t.ID,
			})
		}
		err = c.repoStore.DeleteAllTags(ctx, newDBRepo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to delete database tag", slog.Any("error", err))
		}
		err = c.repoStore.BatchCreateRepoTags(ctx, repoTags)
		if err != nil {
			slog.Error("failed to batch create database tag", slog.Any("error", err))
		}
	}

	err = c.repoStore.DeleteAllFiles(ctx, newDBRepo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to delete all files for repo", slog.Any("error", err))
	}

	ctxGetFileList, cancel := context.WithTimeout(ctx, 5*time.Second)
	files, err := sc.FileList(ctxGetFileList, s)
	cancel()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

		err = c.fileStore.BatchCreate(ctx, dbFiles)
		if err != nil {
			slog.Error("failed to create all files of repo", slog.Any("sync_version", s))
		}
	}

	//create new model record related to repo
	dbMCPServer := database.MCPServer{
		Repository:      newDBRepo,
		RepositoryID:    newDBRepo.ID,
		ToolsNum:        m.ToolsNum,
		Configuration:   m.Configuration,
		Schema:          m.Schema,
		ProgramLanguage: m.ProgramLanguage,
		RunMode:         m.RunMode,
		InstallDepsCmds: m.InstallDepsCmds,
		BuildCmds:       m.BuildCmds,
		LaunchCmds:      m.LaunchCmds,
	}
	_, err = c.mcpStore.CreateIfNotExist(ctx, dbMCPServer)
	if err != nil {
		return fmt.Errorf("failed to create database model, cause: %w", err)
	}

	// create new trending scores related to repo
	if len(m.Scores) != 0 {
		err = c.createLocalRecom(ctx, newDBRepo.ID, m.Scores)
		if err != nil {
			return fmt.Errorf("failed to create database.recom_repo_scores, cause: %w", err)
		}
	}
	return nil
}

func (c *multiSyncComponentImpl) createUser(ctx context.Context, req types.CreateUserRequest) (database.User, error) {
	gsUserReq := gitserver.CreateUserRequest{
		Nickname: req.Name,
		Username: req.Username,
		Email:    req.Email,
	}
	gsUserResp, err := c.gitServer.CreateUser(gsUserReq)
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
		UUID:     req.UUID,
	}
	err = c.userStore.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user,error:%w", err)
		return database.User{}, newError
	}

	return *user, err
}

func (c *multiSyncComponentImpl) getUser(ctx context.Context, userName string) (database.User, error) {
	return c.userStore.FindByUsername(ctx, userName)
}

func (c *multiSyncComponentImpl) createLocalSyncVersion(ctx context.Context, v types.SyncVersion) error {
	syncVersion := database.SyncVersion{
		Version:        v.Version,
		SourceID:       v.SourceID,
		RepoPath:       v.RepoPath,
		RepoType:       v.RepoType,
		LastModifiedAt: v.LastModifyTime,
		ChangeLog:      v.ChangeLog,
		Completed:      false,
	}
	err := c.syncVersionStore.Create(ctx, &syncVersion)
	if err != nil {
		return err
	}
	return nil
}

func (c *multiSyncComponentImpl) createLocalRecom(ctx context.Context, repoID int64, scores []types.WeightScore) error {
	dbRecomRepoScores := make([]*database.RecomRepoScore, len(scores))
	for i, score := range scores {
		dbRecomRepoScores[i] = &database.RecomRepoScore{
			RepositoryID: repoID,
			WeightName:   database.RecomWeightName(score.WeightName),
			Score:        score.Score}
	}
	err := c.recomStore.UpsertScore(ctx, dbRecomRepoScores)
	if err != nil {
		return fmt.Errorf("failed to create recom score, cause: %w", err)
	}
	return nil
}
