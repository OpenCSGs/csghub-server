package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	mirrorcache "opencsg.com/csghub-server/mirror/cache"
)

type mirrorComponentImpl struct {
	repoComp            RepoComponent
	accessTokenStore    database.AccessTokenStore
	modelStore          database.ModelStore
	datasetStore        database.DatasetStore
	codeStore           database.CodeStore
	repoStore           database.RepoStore
	mirrorStore         database.MirrorStore
	mirrorRepoStore     database.MirrorRepoStore
	mirrorSourceStore   database.MirrorSourceStore
	syncVersionStore    database.SyncVersionStore
	mirrorTaskJobStore  database.MirrorTaskJobStore
	mirrorJobClient     workhub.JobClient
	mirrorRepoJobClient database.MirrorJobClient
	namespaceStore      database.NamespaceStore
	userStore           database.UserStore
	config              *config.Config
	// syncCache removes LFS sync cache after mirror deletion.
	syncCacheMu                 sync.Mutex
	syncCache                   mirrorcache.Cache
	mirrorNamespaceMappingStore database.MirrorNamespaceMappingStore
}

type MirrorComponent interface {
	// CreateMirror creates a mirror configuration for an existing repository.
	CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error)
	// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
	CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error)
	// MirrorFromSaas enqueues a mirror sync from the configured OpenCSG SaaS Git source.
	MirrorFromSaas(ctx context.Context, namespace, name string, repoType types.RepositoryType) error
	// GetMirror returns mirror configuration and current task status for an existing repository.
	GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error)
	// UpdateMirror updates mirror configuration for an existing repository.
	UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error)
	// SyncMirror enqueues a new repo sync task for an existing mirror repository.
	SyncMirror(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) error
	// DeleteMirror deletes an existing mirror after cancelling its workhub jobs.
	DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error
	Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error)
	Index(ctx context.Context, per, page int, filter types.MirrorFilter) ([]types.Mirror, int, error)
	Statistics(ctx context.Context) ([]types.MirrorStatusCount, error)
	BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error
	Schedule(ctx context.Context) error
	PublicModelRepo(ctx context.Context) error
	Delete(ctx context.Context, id int64) error
	ResolveNamespace(ctx context.Context, req types.ResolveNamespaceReq) (*types.ResolveNamespaceResp, error)
}

func NewMirrorComponent(config *config.Config) (MirrorComponent, error) {
	var err error
	c := &mirrorComponentImpl{}

	c.repoComp, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component,error:%w", err)
	}
	c.accessTokenStore = database.NewAccessTokenStore()
	c.modelStore = database.NewModelStore()
	c.datasetStore = database.NewDatasetStore()
	c.codeStore = database.NewCodeStore()
	c.repoStore = database.NewRepoStore()
	c.mirrorStore = database.NewMirrorStore()
	jobClient, err := workhub.NewJobClient(context.Background(), database.GetDB().BunDB)
	if err != nil {
		return nil, fmt.Errorf("fail to create mirror repo job client,error:%w", err)
	}
	mirrorRepoJobClient := workhub.NewMirrorRepoJobClient(jobClient)
	c.mirrorJobClient = jobClient
	c.mirrorRepoJobClient = mirrorRepoJobClient
	c.mirrorTaskJobStore = database.NewMirrorTaskJobStore()
	c.mirrorRepoStore = database.NewMirrorRepoStore(mirrorRepoJobClient)
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.syncVersionStore = database.NewSyncVersionStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.config = config
	c.mirrorNamespaceMappingStore = database.NewMirrorNamespaceMappingStore()
	return c, nil
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

// CreateMirror creates a mirror configuration for an existing repository.
func (m *mirrorComponentImpl) CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error) {
	var mirror database.Mirror
	admin, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to create mirror for this repo")
	}

	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if req.MirrorSourceID != 0 {
		mirrorSource, err := m.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get mirror source, err: %w, id: %d", err, req.MirrorSourceID)
		}
		mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	}

	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath

	mirror.RepositoryID = repo.ID
	mirror.Repository = repo

	mirror.Priority = types.ASAPMirrorPriority

	sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceUrl)
	applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	reqMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:       repo,
		CreateRepository: false,
		Mirror:           mirror,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror repo records: %w", err)
	}

	return reqMirror, nil
}

// MirrorFromSaas enqueues one workhub sync for an existing on-prem repository backed by a SaaS Git source.
func (m *mirrorComponentImpl) MirrorFromSaas(ctx context.Context, namespace, name string, repoType types.RepositoryType) error {
	repo, err := m.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if mirror != nil {
		return m.requeueMirrorFromSaas(ctx, repo, mirror)
	}

	syncVersion, err := m.syncVersionStore.FindByRepoTypeAndPath(ctx, repo.PathWithOutPrefix(), repoType)
	if err != nil {
		return fmt.Errorf("failed to find sync version, error: %w", err)
	}
	sourceURL := common.TrimPrefixCloneURLBySourceID(m.config.MultiSync.SaasSyncDomain, string(repoType), namespace, name, syncVersion.SourceID)
	sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(sourceURL)
	applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	reqMirror := database.Mirror{
		SourceUrl:      sourceURL,
		RepositoryID:   repo.ID,
		Repository:     repo,
		SourceRepoPath: fmt.Sprintf("%s/%s", namespace, name),
		Priority:       types.ASAPMirrorPriority,
	}
	if _, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:       repo,
		CreateRepository: false,
		Mirror:           reqMirror,
	}); err != nil {
		return fmt.Errorf("failed to create mirror repo records: %w", err)
	}
	return nil
}

// GetMirror returns mirror configuration and current task status for an existing repository.
func (m *mirrorComponentImpl) GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error) {
	var (
		status      types.MirrorTaskStatus
		progress    int8
		lastMessage string
	)
	admin, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to get mirror for this repo")
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if mirror.CurrentTask != nil {
		status = mirror.CurrentTask.Status
		progress = int8(mirror.CurrentTask.Progress)
		lastMessage = mirror.CurrentTask.ErrorMessage
	}
	resMirror := &types.Mirror{
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
		LocalRepoPath:   fmt.Sprintf("%ss/%s", mirror.Repository.RepositoryType, mirror.Repository.Path),
		LastMessage:     lastMessage,
		Status:          status,
		Progress:        progress,
		CreatedAt:       mirror.CreatedAt,
		UpdatedAt:       mirror.UpdatedAt,
	}
	return resMirror, nil
}

// UpdateMirror updates mirror configuration for an existing repository.
func (m *mirrorComponentImpl) UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error) {
	admin, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to update mirror for this repo")
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}

	pushAccessToken, err := m.accessTokenStore.GetUserGitToken(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find access token, error: %w", err)
	}

	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = req.PushUrl
	mirror.AccessToken = req.AccessToken
	mirror.PushUsername = req.CurrentUser
	mirror.PushAccessToken = pushAccessToken.Token
	mirror.SourceRepoPath = req.SourceRepoPath
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s", req.RepoType, req.Namespace, req.Name)
	err = m.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror, error: %w", err)
	}
	return mirror, nil
}

// SyncMirror validates access to an existing mirror repository and enqueues a new repo sync task.
func (m *mirrorComponentImpl) SyncMirror(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) error {
	user, err := m.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		admin, err := m.repoComp.CheckCurrentUserPermission(ctx, currentUser, namespace, membership.RoleAdmin)
		if err != nil {
			return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
		}

		if !admin {
			return errorx.ErrForbiddenMsg("need be owner or admin role to sync mirror for this repo")
		}
	}
	repo, err := m.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	return requeueMirrorRepoTask(ctx, m.mirrorTaskJobStore, m.mirrorRepoJobClient, m.mirrorJobClient, repo, mirror)
}

// DeleteMirror validates access to an existing mirror repository and deletes it transactionally.
func (m *mirrorComponentImpl) DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error {
	admin, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return fmt.Errorf("users do not have permission to delete mirror for this repo")
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if err := m.mirrorStore.DeleteWithTaskCancelTx(ctx, mirror.ID, m.mirrorJobClient); err != nil {
		return fmt.Errorf("failed to delete mirror, error: %w", err)
	}
	syncCache := ensureRepoSyncCache(ctx, &m.syncCacheMu, &m.syncCache, m.config)
	deleteRepoSyncCache(ctx, syncCache, repo.ID, strconv.Itoa(m.config.Mirror.PartSize))
	return nil
}

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

func (m *mirrorComponentImpl) Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error) {
	var mirrorRepos []types.MirrorRepo
	mirros, total, err := m.mirrorStore.IndexWithPagination(ctx, per, page, types.MirrorFilter{}, true)
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

func (m *mirrorComponentImpl) Index(ctx context.Context, per, page int, filter types.MirrorFilter) ([]types.Mirror, int, error) {
	var mirrorsResp []types.Mirror
	mirrors, total, err := m.mirrorStore.IndexWithPagination(ctx, per, page, filter, false)
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

func (m *mirrorComponentImpl) Statistics(ctx context.Context) ([]types.MirrorStatusCount, error) {
	var scs []types.MirrorStatusCount
	statusCounts, err := m.mirrorStore.StatusCount(ctx)
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

func (m *mirrorComponentImpl) BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error {
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
	existingMirrors, err := m.mirrorStore.FindBySourceURLs(ctx, sourceURLs)
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
		err = m.mirrorStore.BatchUpdate(ctx, toBeUpdatedMirrors)
		if err != nil {
			return fmt.Errorf("failed to batch update mirrors: %v", err)
		}
	}

	if len(toBeCreatedMirrors) > 0 {
		err = m.mirrorStore.BatchCreate(ctx, toBeCreatedMirrors)
		if err != nil {
			return fmt.Errorf("failed to batch create mirrors: %v", err)
		}
	}

	return nil
}

func (m *mirrorComponentImpl) Delete(ctx context.Context, id int64) error {
	mirror, err := m.mirrorStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find mirror: %w", err)
	}
	if err := m.mirrorStore.DeleteWithTaskCancelTx(ctx, id, m.mirrorJobClient); err != nil {
		return fmt.Errorf("failed to delete mirror: %w", err)
	}
	syncCache := ensureRepoSyncCache(ctx, &m.syncCacheMu, &m.syncCache, m.config)
	deleteRepoSyncCache(ctx, syncCache, mirror.RepositoryID, strconv.Itoa(m.config.Mirror.PartSize))
	return nil
}

// ensureRepoSyncCache lazily creates the Redis cache used by mirror cleanup paths.
func ensureRepoSyncCache(ctx context.Context, mu *sync.Mutex, syncCache *mirrorcache.Cache, cfg *config.Config) mirrorcache.Cache {
	if syncCache == nil || *syncCache != nil {
		if syncCache == nil {
			return nil
		}
		return *syncCache
	}
	mu.Lock()
	defer mu.Unlock()
	if *syncCache != nil {
		return *syncCache
	}
	if cfg == nil {
		return nil
	}
	cache, err := mirrorcache.NewCache(context.Background(), cfg)
	if err != nil {
		slog.WarnContext(ctx, "failed to create mirror sync cache for cleanup", slog.Any("error", err))
		return nil
	}
	*syncCache = cache
	return cache
}

// deleteRepoSyncCache removes LFS cache for a repository after its mirror task is stopped or deleted.
func deleteRepoSyncCache(ctx context.Context, syncCache mirrorcache.Cache, repoID int64, partSize string) {
	if syncCache == nil || repoID == 0 {
		return
	}
	if err := syncCache.DeleteRepoSyncCache(ctx, repoID, partSize); err != nil {
		slog.WarnContext(ctx, "failed to delete mirror repo sync cache", slog.Any("error", err), slog.Int64("repo_id", repoID))
	}
}
