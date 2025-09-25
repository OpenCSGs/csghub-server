package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/cache"
)

type mirrorComponentImpl struct {
	tokenStore                  database.GitServerAccessTokenStore
	mirrorServer                mirrorserver.MirrorServer
	saas                        bool
	repoComp                    RepoComponent
	git                         gitserver.GitServer
	s3Client                    s3.Client
	lfsBucket                   string
	modelStore                  database.ModelStore
	datasetStore                database.DatasetStore
	codeStore                   database.CodeStore
	repoStore                   database.RepoStore
	mirrorStore                 database.MirrorStore
	mirrorSourceStore           database.MirrorSourceStore
	namespaceStore              database.NamespaceStore
	lfsMetaObjectStore          database.LfsMetaObjectStore
	mcpServerStore              database.MCPServerStore
	userStore                   database.UserStore
	config                      *config.Config
	syncCache                   cache.Cache
	mirrorTaskStore             database.MirrorTaskStore
	mirrorNamespaceMappingStore database.MirrorNamespaceMappingStore
}

type MirrorComponent interface {
	// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
	CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error)
	Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error)
	Index(ctx context.Context, per, page int, search string) ([]types.Mirror, int, error)
	Statistics(ctx context.Context) ([]types.MirrorStatusCount, error)
	BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error
	Schedule(ctx context.Context) error
	ListQueue(ctx context.Context, count int64) (types.MirrorListResp, error)
	PublicModelRepo(ctx context.Context) error
	Delete(ctx context.Context, id int64) error
}

func NewMirrorComponent(config *config.Config) (MirrorComponent, error) {
	var err error
	c := &mirrorComponentImpl{}
	if config.GitServer.Type == types.GitServerTypeGitea {
		c.mirrorServer, err = git.NewMirrorServer(config)
		if err != nil {
			newError := fmt.Errorf("fail to create git mirror server,error:%w", err)
			slog.Error(newError.Error())
			return nil, newError
		}
	}

	c.repoComp, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component,error:%w", err)
	}
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.lfsBucket = config.S3.Bucket
	c.modelStore = database.NewModelStore()
	c.datasetStore = database.NewDatasetStore()
	c.codeStore = database.NewCodeStore()
	c.repoStore = database.NewRepoStore()
	c.mirrorStore = database.NewMirrorStore()
	c.tokenStore = database.NewGitServerAccessTokenStore()
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.userStore = database.NewUserStore()
	c.mcpServerStore = database.NewMCPServerStore()
	c.mirrorTaskStore = database.NewMirrorTaskStore()
	c.saas = config.Saas
	c.config = config
	c.mirrorNamespaceMappingStore = database.NewMirrorNamespaceMappingStore()
	syncCache, err := cache.NewCache(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	c.syncCache = syncCache
	return c, nil
}

// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
func (c *mirrorComponentImpl) CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error) {
	var (
		username string
		err      error
	)
	namespace := c.mapNamespaceAndName(req.SourceNamespace)
	name := req.SourceName
	repo, err := c.repoStore.FindByMirrorSourceURL(ctx, req.SourceGitCloneUrl)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to find repo by source url, error: %w", err)
	}

	if repo != nil && repo.ID != 0 {
		namespace, name, err := common.GetNamespaceAndNameFromPath(repo.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespace and name: %w", err)
		}
		err = c.repoComp.SyncMirror(ctx, req.RepoType, namespace, name, req.CurrentUser)
		if err != nil {
			return nil, fmt.Errorf("failed to sync mirror repo, error: %w", err)
		}
		return &database.Mirror{RepositoryID: repo.ID}, nil
	}
	repo, err = c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check repo existence, error: %w", err)
	}
	if repo != nil && repo.ID != 0 {
		name = fmt.Sprintf("%s_%s", req.SourceNamespace, req.SourceName)
		repo, err = c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to check repo existence, error: %w", err)
		}
		if repo != nil && repo.ID != 0 {
			err := fmt.Errorf(
				"repo already exists, repo type: %s, source namespace: %s, source name: %s, target namespace: %s, target name: %s",
				req.RepoType, req.SourceNamespace, req.SourceName, namespace, name)
			return &database.Mirror{RepositoryID: repo.ID}, errorx.DuplicateKey(err,
				errorx.Ctx().
					Set("repo type", req.RepoType).
					Set("source namespace", req.SourceNamespace).
					Set("source name", req.SourceName).
					Set("target namespace", namespace).
					Set("target name", name),
			)
		}
	}

	dbNamespace, err := c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("namespace does not exist, namespace: %s", namespace)
	}
	username = dbNamespace.User.Username

	// create repo, create mirror repo
	_, repo, err = c.repoComp.CreateRepo(ctx, types.CreateRepoReq{
		Username:  username,
		Namespace: namespace,
		Name:      name,
		Nickname:  name,
		//TODO: tranlate description automatically
		Description: req.Description,
		//only mirror public repository
		Private:       true,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
		ToolCount:     len(req.MCPServerAttributes.Tools),
		StarCount:     req.MCPServerAttributes.StarCount,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create OpenCSG repo, error: %w", err)
	}
	repoPath := path.Join(namespace, name)

	if req.RepoType == types.ModelRepo {
		dbModel := database.Model{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.modelStore.CreateAndUpdateRepoPath(ctx, dbModel, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create model, error: %w", err)
		}
	} else if req.RepoType == types.DatasetRepo {
		dbDataset := database.Dataset{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.datasetStore.CreateAndUpdateRepoPath(ctx, dbDataset, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create dataset, error: %w", err)
		}
	} else if req.RepoType == types.CodeRepo {
		dbCode := database.Code{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.codeStore.CreateAndUpdateRepoPath(ctx, dbCode, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create code, error: %w", err)
		}
	} else if req.RepoType == types.MCPServerRepo {
		configuration, err := json.Marshal(req.MCPServerAttributes.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mcp configuration: %w", err)
		}

		tools, err := json.Marshal(struct {
			Tools []types.MCPTool `json:"tools"`
		}{
			Tools: req.MCPServerAttributes.Tools,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mcp tools: %w", err)
		}
		dbMCPServer := database.MCPServer{
			Repository:    repo,
			RepositoryID:  repo.ID,
			ToolsNum:      len(req.MCPServerAttributes.Tools),
			Configuration: string(configuration),
			Schema:        string(tools),
			AvatarURL:     req.MCPServerAttributes.AvatarURL,
		}

		mcpServer, err := c.mcpServerStore.CreateAndUpdateRepoPath(ctx, dbMCPServer, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create mcp server, error: %w", err)
		}

		for _, tool := range req.MCPServerAttributes.Tools {
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool input schema: %w", err)
			}
			mcpServerProperty := database.MCPServerProperty{
				MCPServerID: mcpServer.ID,
				Kind:        types.MCPPropTool,
				Name:        tool.Name,
				Description: tool.Description,
				Schema:      string(schema),
			}
			_, err = c.mcpServerStore.AddProperty(ctx, mcpServerProperty)
			if err != nil {
				return nil, fmt.Errorf("failed to add property to mcp server: %w", err)
			}
		}
	}

	var mirror database.Mirror

	if req.MirrorSourceID != 0 {
		mirrorSource, err := c.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to find mirror source, error: %w", err)
		}
		mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.SourceNamespace, req.SourceName)
	}
	// mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceGitCloneUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.SourceNamespace
	mirror.RepositoryID = repo.ID
	mirror.Repository = repo
	mirror.SourceRepoPath = fmt.Sprintf("%s/%s", req.SourceNamespace, req.SourceName)
	mirror.Priority = types.ASAPMirrorPriority

	sourceType, sourcePath, err := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
	if err == nil {
		err = c.repoStore.UpdateSourcePath(ctx, repo.ID, sourcePath, sourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to update source path in repo: %v", err)
		}
	}

	reqMirror, err := c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror")
	}

	reqMirror.Status = types.MirrorQueued
	err = c.mirrorStore.Update(ctx, reqMirror)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror status: %v", err)
	}
	mt := database.MirrorTask{
		MirrorID: reqMirror.ID,
		Status:   types.MirrorQueued,
		Priority: mirror.Priority,
	}
	_, err = c.mirrorTaskStore.Create(ctx, mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror task: %v", err)
	}

	return reqMirror, nil

}

func (m *mirrorComponentImpl) mapNamespaceAndName(sourceNamespace string) string {
	n, err := m.mirrorNamespaceMappingStore.FindBySourceNamespace(context.Background(), sourceNamespace)
	if err != nil {
		return "AIWizards"
	}
	if n.TargetNamespace == "" {
		return "AIWizards"
	}
	return n.TargetNamespace
}

// var mirrorStatusAndRepoSyncStatusMapping = map[types.MirrorTaskStatus]types.RepositorySyncStatus{
// 	types.MirrorQueued:           types.SyncStatusPending,
// 	types.MirrorRepoSyncStart:    types.SyncStatusInProgress,
// 	types.MirrorRepoSyncFinished: types.SyncStatusInProgress,
// 	types.MirrorRepoSyncFailed:   types.SyncStatusFailed,
// 	types.MirrorRepoSyncFatal:    types.SyncStatusFailed,
// 	types.MirrorLfsSyncStart:     types.SyncStatusInProgress,
// 	types.MirrorLfsSyncFinished:  types.SyncStatusInProgress,
// 	types.MirrorLfsSyncFailed:    types.SyncStatusFailed,
// 	types.MirrorLfsSyncFatal:     types.SyncStatusFailed,
// }

func getAllFiles(ctx context.Context, namespace, repoName, folder string, repoType types.RepositoryType, ref string, gsTree func(ctx context.Context, req types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)) ([]*types.File, error) {
	var (
		files  []*types.File
		cursor string
	)

	for {
		resp, err := gsTree(ctx, types.GetTreeRequest{
			Path:      folder,
			Namespace: namespace,
			Name:      repoName,
			RepoType:  repoType,
			Ref:       ref,
			Recursive: true,
			Limit:     types.MaxFileTreeSize,
			Cursor:    cursor,
		})

		if resp == nil {
			break
		}

		cursor = resp.Cursor
		if err != nil {
			return files, fmt.Errorf("failed to get repo %s/%s/%s file tree,%w", repoType, namespace, repoName, err)
		}

		for _, file := range resp.Files {
			if file.Type == "dir" {
				continue
			}
			files = append(files, file)
		}

		if resp.Cursor == "" {
			break
		}
	}
	return files, nil
}

func (c *mirrorComponentImpl) Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error) {
	var mirrorRepos []types.MirrorRepo
	mirros, total, err := c.mirrorStore.IndexWithPagination(ctx, per, page, "", true)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror repositories: %v", err)
	}
	for _, m := range mirros {
		if m.Repository != nil && m.CurrentTask != nil {
			RepoSyncStatus := common.MirrorTaskStatusToRepoStatus(m.CurrentTask.Status)
			mirrorRepos = append(mirrorRepos, types.MirrorRepo{
				ID:         m.ID,
				Path:       m.Repository.Path,
				SyncStatus: RepoSyncStatus,
				RepoType:   m.Repository.RepositoryType,
				Progress:   int8(m.CurrentTask.Progress),
			})
		}
	}
	return mirrorRepos, total, nil
}

func (c *mirrorComponentImpl) Index(ctx context.Context, per, page int, search string) ([]types.Mirror, int, error) {
	var mirrorsResp []types.Mirror
	mirrors, total, err := c.mirrorStore.IndexWithPagination(ctx, per, page, search, false)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror mirrors: %v", err)
	}
	for _, mirror := range mirrors {
		var status types.MirrorTaskStatus
		if mirror.CurrentTask != nil {
			status = mirror.CurrentTask.Status
		} else if mirror.Status == "" {
			status = types.MirrorQueued
		} else {
			status = mirror.Status
		}
		mirrorsResp = append(mirrorsResp, types.Mirror{
			ID:        mirror.ID,
			SourceUrl: mirror.SourceUrl,
			MirrorSource: types.MirrorSource{
				SourceName: mirror.MirrorSource.SourceName,
			},
			Username:        mirror.Username,
			AccessToken:     mirror.AccessToken,
			PushUrl:         mirror.PushUrl,
			PushUsername:    mirror.PushUsername,
			PushAccessToken: mirror.PushAccessToken,
			LastUpdatedAt:   mirror.LastUpdatedAt,
			SourceRepoPath:  mirror.SourceRepoPath,
			LocalRepoPath:   mirror.RepoPath(),
			LastMessage:     mirror.LastMessage,
			Status:          status,
			Progress:        mirror.Progress,
		})
	}
	return mirrorsResp, total, nil
}

func (c *mirrorComponentImpl) Statistics(ctx context.Context) ([]types.MirrorStatusCount, error) {
	var scs []types.MirrorStatusCount
	statusCounts, err := c.mirrorStore.StatusCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror statistics: %v", err)
	}

	for _, statusCount := range statusCounts {
		scs = append(scs, types.MirrorStatusCount{
			Status: statusCount.Status,
			Count:  statusCount.Count,
		})
	}

	return scs, nil
}

func (c *mirrorComponentImpl) BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error {
	var (
		toBeUpdatedMirrors []database.Mirror
		toBeCreatedMirrors []database.Mirror
		sourceURLs         []string
		existingSourceURLs []string
	)
	sourceURLMirrorMapping := make(map[string]types.MirrorReq)
	for _, mirror := range req.Mirrors {
		sourceURLMirrorMapping[mirror.SourceURL] = mirror
		sourceURLs = append(sourceURLs, mirror.SourceURL)
	}
	existingMirrors, err := c.mirrorStore.FindBySourceURLs(ctx, sourceURLs)
	if err != nil {
		return fmt.Errorf("failed to get mirrors: %v", err)
	}
	for _, eMirror := range existingMirrors {
		if mirror, ok := sourceURLMirrorMapping[eMirror.SourceUrl]; ok {
			if mirror.Priority == 0 {
				mirror.Priority = 1
			}
			eMirror.Priority = types.MirrorPriority(mirror.Priority)
			eMirror.RemoteUpdatedAt = mirror.UpdatedAt
			toBeUpdatedMirrors = append(toBeUpdatedMirrors, eMirror)
		}
		existingSourceURLs = append(existingSourceURLs, eMirror.SourceUrl)
	}
	for _, mirror := range req.Mirrors {
		if mirror.Priority == 0 {
			mirror.Priority = 1
		}
		if !slices.Contains(existingSourceURLs, mirror.SourceURL) {
			dbMirror := database.Mirror{
				SourceUrl:       mirror.SourceURL,
				Priority:        types.MirrorPriority(mirror.Priority),
				RemoteUpdatedAt: mirror.UpdatedAt,
				RepositoryID:    0,
				Status:          types.MirrorQueued,
				MirrorSourceID:  mirror.SourceID,
			}
			toBeCreatedMirrors = append(toBeCreatedMirrors, dbMirror)
		}
	}

	if len(toBeUpdatedMirrors) > 0 {
		err = c.mirrorStore.BatchUpdate(ctx, toBeUpdatedMirrors)
		if err != nil {
			return fmt.Errorf("failed to batch update mirrors: %v", err)
		}
	}

	if len(toBeCreatedMirrors) > 0 {
		err = c.mirrorStore.BatchCreate(ctx, toBeCreatedMirrors)
		if err != nil {
			return fmt.Errorf("failed to batch create mirrors: %v", err)
		}
	}

	return nil
}

func (c *mirrorComponentImpl) ListQueue(ctx context.Context, count int64) (types.MirrorListResp, error) {
	var (
		resp      types.MirrorListResp
		mirrorIDs []int64
	)
	mirrorMap := make(map[int64]database.Mirror)
	// lfsmirrorIDs := c.mq.ListLfsMirrorTasks(count)
	// repoMirrorIDs := c.mq.ListRepoMirrorTasks(count)

	runningTasks, err := c.syncCache.GetRunningTask(ctx)
	if err != nil {
		return resp, fmt.Errorf("failed to get running tasks: %w", err)
	}
	for _, id := range runningTasks {
		mirrorIDs = append(mirrorIDs, id)
	}

	// mirrorIDs = append(mirrorIDs, lfsmirrorIDs...)
	// mirrorIDs = append(mirrorIDs, repoMirrorIDs...)
	mirrors, err := c.mirrorStore.FindByIDs(ctx, mirrorIDs)
	if err != nil {
		return resp, fmt.Errorf("failed to find mirror by ids: %w", err)
	}
	for _, mirror := range mirrors {
		mirrorMap[mirror.ID] = mirror
	}
	// for _, id := range lfsmirrorIDs {
	// 	if mirror, ok := mirrorMap[id]; ok {
	// 		resp.LfsMirrorTasks = append(resp.LfsMirrorTasks, types.MirrorTask{
	// 			MirrorID:  mirror.ID,
	// 			SourceUrl: mirror.SourceUrl,
	// 			RepoPath:  mirror.RepoPath(),
	// 			Priority:  int(mirror.Priority),
	// 		})
	// 	}
	// }

	// for _, id := range repoMirrorIDs {
	// 	if mirror, ok := mirrorMap[id]; ok {
	// 		resp.RepoMirrorTasks = append(resp.RepoMirrorTasks, types.MirrorTask{
	// 			MirrorID:  mirror.ID,
	// 			SourceUrl: mirror.SourceUrl,
	// 			RepoPath:  mirror.RepoPath(),
	// 			Priority:  int(mirror.Priority),
	// 		})
	// 	}
	// }

	resp.RunningTasks = make(map[int]types.MirrorTask)

	for k, mirrorID := range runningTasks {
		if mirror, ok := mirrorMap[mirrorID]; ok {
			resp.RunningTasks[k] = types.MirrorTask{
				MirrorID:  mirror.ID,
				SourceUrl: mirror.SourceUrl,
				RepoPath:  mirror.RepoPath(),
				Priority:  int(mirror.Priority),
			}
		}
	}

	return resp, nil
}

func (c *mirrorComponentImpl) Delete(ctx context.Context, id int64) error {
	m, err := c.mirrorStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find mirror by id: %w", err)
	}

	err = c.mirrorStore.Delete(ctx, m)
	if err != nil {
		return fmt.Errorf("failed to delete mirror: %w", err)
	}

	return nil
}
