package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	mirrorcache "opencsg.com/csghub-server/mirror/cache"
)

type mirrorComponentImpl struct {
	repoComp                    RepoComponent
	createRepoForAuthorizedUser authorizedRepoCreator
	accessTokenStore            database.AccessTokenStore
	modelStore                  database.ModelStore
	datasetStore                database.DatasetStore
	codeStore                   database.CodeStore
	repoStore                   database.RepoStore
	mirrorStore                 database.MirrorStore
	mirrorRepoStore             database.MirrorRepoStore
	mirrorSourceStore           database.MirrorSourceStore
	syncVersionStore            database.SyncVersionStore
	mirrorTaskJobStore          database.MirrorTaskJobStore
	mirrorJobClient             workhub.JobClient
	mirrorRepoJobClient         database.MirrorJobClient
	namespaceStore              database.NamespaceStore
	userStore                   database.UserStore
	orgStore                    database.OrgStore
	memberStore                 database.MemberStore
	rebac                       rebac.Authorizer
	config                      *config.Config
	// syncCache removes LFS sync cache after mirror deletion.
	syncCacheMu                 sync.Mutex
	syncCache                   mirrorcache.Cache
	mirrorNamespaceMappingStore database.MirrorNamespaceMappingStore
	// mirrorMetadataClientFactory creates source-specific clients used to import MCP and skill metadata.
	mirrorMetadataClientFactory mirrorMetadataClientFactory
}

type authorizedRepoCreateRequest struct {
	UserID int64
	Repo   types.CreateRepoReq
}

type authorizedRepoCreator func(context.Context, authorizedRepoCreateRequest) (*gitserver.CreateRepoResp, *database.Repository, *gitserver.CommitFilesReq, error)

type MirrorComponent interface {
	// CreateMirror creates a mirror configuration for an existing repository.
	CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error)
	// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
	CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error)
	// MirrorFromSaas enqueues a mirror sync from the configured OpenCSG SaaS Git source.
	MirrorFromSaas(ctx context.Context, req types.MirrorFromSaasReq) (*types.MirrorFromSaasResponse, error)
	// MirrorFromSaasStatus returns public progress for the current SaaS mirror task.
	MirrorFromSaasStatus(ctx context.Context, req types.MirrorFromSaasStatusReq) (*types.MirrorSyncStatusResponse, error)
	// GetMirror returns mirror configuration and current task status for an existing repository.
	GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error)
	// UpdateMirror updates mirror configuration for an existing repository.
	UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error)
	// SyncMirror enqueues a new repo sync task for an existing mirror repository.
	SyncMirror(ctx context.Context, req types.SyncMirrorReq) error
	// DeleteMirror deletes an existing mirror after cancelling its workhub jobs.
	DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error
	// ListMirrorSyncs returns mirror synchronization summaries from persisted task state.
	ListMirrorSyncs(ctx context.Context, req types.MirrorSyncListReq) (*types.MirrorSyncListResponse, error)
	Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error)
	Index(ctx context.Context, per, page int, filter types.MirrorFilter) ([]types.Mirror, int, error)
	Statistics(ctx context.Context) ([]types.MirrorStatusCount, error)
	Schedule(ctx context.Context) error
	PublicModelRepo(ctx context.Context) error
	Delete(ctx context.Context, id int64) error
	ResolveNamespace(ctx context.Context, req types.ResolveNamespaceReq) (*types.ResolveNamespaceResp, error)
}

// mirrorSyncResolution is the internal effective state used to build synchronization list rows.
type mirrorSyncResolution struct {
	Phase     types.MirrorSyncPhase
	Status    types.MirrorSyncOverallStatus
	Result    types.MirrorSyncResult
	Retrying  bool
	RepoStage types.MirrorSyncStageSummary
	LFSStage  types.MirrorSyncStageSummary
}

// resolveMirrorSyncStatus maps the persisted mirror task state to the public lifecycle.
func resolveMirrorSyncStatus(mirror database.Mirror) mirrorSyncResolution {
	task := mirror.CurrentTask
	if task == nil {
		return noTaskMirrorSyncResolution()
	}
	if mirror.CurrentTaskID == 0 || task.ID != mirror.CurrentTaskID || task.MirrorID != mirror.ID {
		return invalidMirrorSyncResolution()
	}

	switch task.Status {
	case types.MirrorQueued:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseRepo, Status: types.MirrorSyncOverallWaiting,
			RepoStage: notStartedMirrorSyncStage(), LFSStage: notStartedMirrorSyncStage(),
		}
	case types.MirrorRepoSyncStart:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseRepo, Status: types.MirrorSyncOverallRunning,
			RepoStage: runningMirrorSyncStage(), LFSStage: notStartedMirrorSyncStage(),
		}
	case types.MirrorRepoSyncFailed:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseRepo, Status: types.MirrorSyncOverallRunning, Retrying: true,
			RepoStage: runningMirrorSyncStage(), LFSStage: notStartedMirrorSyncStage(),
		}
	case types.MirrorRepoSyncFatal:
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultFailed,
			finishedMirrorSyncStage(types.MirrorSyncResultFailed),
			notStartedMirrorSyncStage(),
		)
	case types.MirrorRepoSyncFinished:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseLFS, Status: types.MirrorSyncOverallWaiting,
			RepoStage: finishedMirrorSyncStage(types.MirrorSyncResultSuccess), LFSStage: notStartedMirrorSyncStage(),
		}
	case types.MirrorLfsSyncStart:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseLFS, Status: types.MirrorSyncOverallRunning,
			RepoStage: finishedMirrorSyncStage(types.MirrorSyncResultSuccess), LFSStage: runningMirrorSyncStage(),
		}
	case types.MirrorLfsSyncFailed:
		return mirrorSyncResolution{
			Phase: types.MirrorSyncPhaseLFS, Status: types.MirrorSyncOverallRunning, Retrying: true,
			RepoStage: finishedMirrorSyncStage(types.MirrorSyncResultSuccess), LFSStage: runningMirrorSyncStage(),
		}
	case types.MirrorLfsSyncFinished:
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultSuccess,
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
		)
	case types.MirrorLfsSyncFatal:
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultFailed,
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
			finishedMirrorSyncStage(types.MirrorSyncResultFailed),
		)
	case types.MirrorLfsIncomplete:
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultIncomplete,
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
			finishedMirrorSyncStage(types.MirrorSyncResultIncomplete),
		)
	case types.MirrorRepoTooLarge:
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultTooLarge,
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
			finishedMirrorSyncStage(types.MirrorSyncResultTooLarge),
		)
	case types.MirrorCanceled:
		if task.LFSJobID == 0 {
			return finishedMirrorSyncResolution(
				types.MirrorSyncResultCancelled,
				finishedMirrorSyncStage(types.MirrorSyncResultCancelled),
				notStartedMirrorSyncStage(),
			)
		}
		return finishedMirrorSyncResolution(
			types.MirrorSyncResultCancelled,
			finishedMirrorSyncStage(types.MirrorSyncResultSuccess),
			finishedMirrorSyncStage(types.MirrorSyncResultCancelled),
		)
	default:
		return invalidMirrorSyncResolution()
	}
}

// noTaskMirrorSyncResolution returns a nonterminal-free state for a mirror without a current task.
func noTaskMirrorSyncResolution() mirrorSyncResolution {
	return mirrorSyncResolution{
		Status:    types.MirrorSyncOverallNoTask,
		RepoStage: notStartedMirrorSyncStage(),
		LFSStage:  notStartedMirrorSyncStage(),
	}
}

// invalidMirrorSyncResolution returns a terminal diagnostic result instead of waiting forever.
func invalidMirrorSyncResolution() mirrorSyncResolution {
	return mirrorSyncResolution{
		Phase: types.MirrorSyncPhaseDone, Status: types.MirrorSyncOverallFinished, Result: types.MirrorSyncResultStateInvalid,
		RepoStage: finishedMirrorSyncStage(types.MirrorSyncResultStateInvalid),
		LFSStage:  notStartedMirrorSyncStage(),
	}
}

// finishedMirrorSyncResolution creates one terminal overall result.
func finishedMirrorSyncResolution(result types.MirrorSyncResult, repoStage, lfsStage types.MirrorSyncStageSummary) mirrorSyncResolution {
	return mirrorSyncResolution{
		Phase: types.MirrorSyncPhaseDone, Status: types.MirrorSyncOverallFinished, Result: result,
		RepoStage: repoStage, LFSStage: lfsStage,
	}
}

// notStartedMirrorSyncStage creates a stage that has not run yet.
func notStartedMirrorSyncStage() types.MirrorSyncStageSummary {
	return types.MirrorSyncStageSummary{State: types.MirrorSyncStageNotStarted}
}

// runningMirrorSyncStage creates an active or retrying stage.
func runningMirrorSyncStage() types.MirrorSyncStageSummary {
	return types.MirrorSyncStageSummary{State: types.MirrorSyncStageRunning}
}

// finishedMirrorSyncStage creates a terminal stage with its final result.
func finishedMirrorSyncStage(result types.MirrorSyncResult) types.MirrorSyncStageSummary {
	return types.MirrorSyncStageSummary{State: types.MirrorSyncStageFinished, Result: result}
}

// buildMirrorSyncSummary creates one public synchronization list row.
func buildMirrorSyncSummary(mirror database.Mirror, status mirrorSyncResolution, configuredMaxRetryCount int) types.MirrorSyncSummary {
	var (
		taskID     int64
		priority   types.MirrorPriority
		isUrgent   bool
		progress   int
		retryCount int
	)
	if mirror.CurrentTask != nil {
		taskID = mirror.CurrentTask.ID
		priority = mirror.CurrentTask.Priority
		isUrgent = mirror.CurrentTask.IsUrgent
		progress = mirror.CurrentTask.Progress
		retryCount = mirror.CurrentTask.RetryCount
	}

	var repoPath string
	if mirror.Repository != nil {
		repoPath = mirror.RepoPath()
	}
	return types.MirrorSyncSummary{
		MirrorID: mirror.ID, RepositoryID: mirror.RepositoryID, TaskID: taskID,
		SourceURL: mirror.SourceUrl, Username: mirror.Username,
		AccessToken: maskMirrorSyncToken(mirror.AccessToken), RepoPath: repoPath,
		Priority: priority, IsUrgent: isUrgent, Progress: progress,
		RetryCount: retryCount, MaxRetryCount: configuredMaxRetryCount,
		Status: status.Status, Result: status.Result, Retrying: status.Retrying,
		RepoStage: status.RepoStage, LFSStage: status.LFSStage,
	}
}

// maskMirrorSyncToken returns a stable prefix without exposing a complete source token.
func maskMirrorSyncToken(token string) string {
	if token == "" {
		return ""
	}
	visible := 4
	if len(token) < visible {
		visible = len(token)
	}
	return token[:visible] + "********"
}

// ListMirrorSyncs returns one page of mirror synchronization summaries.
func (m *mirrorComponentImpl) ListMirrorSyncs(ctx context.Context, req types.MirrorSyncListReq) (*types.MirrorSyncListResponse, error) {
	const defaultMirrorSyncPageSize = 10
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Per <= 0 {
		req.Per = defaultMirrorSyncPageSize
	}
	var statuses []types.MirrorTaskStatus
	switch req.Status {
	case "":
	case types.MirrorSyncOverallWaiting:
		statuses = []types.MirrorTaskStatus{
			types.MirrorQueued,
			types.MirrorRepoSyncFinished,
		}
	case types.MirrorSyncOverallRunning:
		statuses = []types.MirrorTaskStatus{
			types.MirrorRepoSyncStart,
			types.MirrorRepoSyncFailed,
			types.MirrorLfsSyncStart,
			types.MirrorLfsSyncFailed,
		}
	case types.MirrorSyncOverallFinished:
		statuses = []types.MirrorTaskStatus{
			types.MirrorRepoSyncFatal,
			types.MirrorLfsSyncFinished,
			types.MirrorLfsSyncFatal,
			types.MirrorLfsIncomplete,
			types.MirrorCanceled,
			types.MirrorRepoTooLarge,
		}
	default:
		return nil, errorx.BadRequest(errors.New("unsupported mirror sync status"), nil)
	}

	query := database.MirrorSyncListQuery{Page: req.Page, Per: req.Per, Search: req.Search, Statuses: statuses}
	mirrors, total, err := m.mirrorStore.IndexSyncWithPagination(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list mirror syncs: %w", err)
	}
	items := m.buildMirrorSyncSummaries(mirrors)
	return &types.MirrorSyncListResponse{Items: items, Total: total, Page: req.Page, Per: req.Per}, nil
}

// buildMirrorSyncSummaries builds list rows from persisted mirror task state.
func (m *mirrorComponentImpl) buildMirrorSyncSummaries(mirrors []database.Mirror) []types.MirrorSyncSummary {
	items := make([]types.MirrorSyncSummary, 0, len(mirrors))
	for _, mirror := range mirrors {
		items = append(items, buildMirrorSyncSummary(mirror, resolveMirrorSyncStatus(mirror), m.config.Mirror.MaxRetryCount))
	}
	return items
}

// NewMirrorComponent shares its River producer with mirror records and namespace permission Stores.
func NewMirrorComponent(config *config.Config) (MirrorComponent, error) {
	var err error
	c := &mirrorComponentImpl{}

	repoComponent, err := NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component,error:%w", err)
	}
	c.repoComp = repoComponent
	c.createRepoForAuthorizedUser = repoComponent.createRepoForAuthorizedUser
	c.rebac = repoComponent.rebac
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
	mirrorRepoJobClient := workhub.NewMirrorRepoJobClient(jobClient, workhub.MirrorJobClientConfig{MaxRetryCount: config.Mirror.MaxRetryCount})
	c.mirrorJobClient = jobClient
	c.mirrorRepoJobClient = mirrorRepoJobClient
	c.mirrorTaskJobStore = database.NewMirrorTaskJobStore()
	tupleJobs := workhub.NewRebacTupleJobClient(jobClient)
	// Mirror targets are standalone organizations and must not claim the hierarchy root slot.
	c.orgStore = database.NewOrgStoreWithJobClients(false, nil, nil, tupleJobs)
	c.mirrorRepoStore = database.NewMirrorRepoStoreWithJobClients(database.GetDB(), mirrorRepoJobClient, tupleJobs)
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.syncVersionStore = database.NewSyncVersionStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.memberStore = database.NewMemberStore()
	c.config = config
	c.mirrorNamespaceMappingStore = database.NewMirrorNamespaceMappingStore()
	c.mirrorMetadataClientFactory = multisync.FromOpenCSG
	return c, nil
}

// mapNamespaceAndName resolves a remote namespace to its mapped target. Only a
// missing mapping uses the caller-selected fallback; store failures propagate.
func (m *mirrorComponentImpl) mapNamespaceAndName(ctx context.Context, sourceNamespace, fallbackNamespace string) (string, error) {
	n, err := m.mirrorNamespaceMappingStore.FindBySourceNamespace(ctx, sourceNamespace)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to find mirror namespace mapping: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) || n == nil {
		return strings.TrimSpace(fallbackNamespace), nil
	}
	if target := strings.TrimSpace(n.TargetNamespace); target != "" {
		return target, nil
	}
	return strings.TrimSpace(fallbackNamespace), nil
}

// CreateMirror creates a mirror configuration for an existing repository.
func (m *mirrorComponentImpl) CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error) {
	priority, err := normalizeMirrorPriority(req.Priority)
	if err != nil {
		return nil, err
	}
	req.Priority = priority
	sourceURL, username, accessToken, err := normalizeMirrorSource(req.SourceUrl, req.Username, req.AccessToken)
	if err != nil {
		return nil, err
	}
	req.SourceUrl = sourceURL
	req.Username = username
	req.AccessToken = accessToken
	var mirror database.Mirror
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	admin, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to create mirror for this repo")
	}

	existingMirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to find mirror by target repo, error: %w", err)
	}
	// Repeated creation requeues the existing source but rejects replacing it implicitly.
	if existingMirror != nil && existingMirror.ID != 0 {
		if existingMirror.SourceUrl != req.SourceUrl {
			return &database.Mirror{RepositoryID: repo.ID}, errorx.MirrorSourceConflict(
				fmt.Errorf("target repo already has mirror source url: %s", existingMirror.SourceUrl),
				errorx.Ctx().
					Set("repo type", req.RepoType).
					Set("target namespace", req.Namespace).
					Set("target name", req.Name).
					Set("source url", req.SourceUrl),
			)
		}
		var usernamePtr, accessTokenPtr *string
		if req.Username != "" {
			usernamePtr = &req.Username
			accessTokenPtr = &req.AccessToken
		}
		if _, err := m.requeueMirrorRepoTask(ctx, repo, existingMirror, usernamePtr, accessTokenPtr, req.Priority, req.Urgent); err != nil {
			return nil, fmt.Errorf("failed to sync mirror repo, error: %w", err)
		}
		if req.Username != "" {
			existingMirror.Username = req.Username
			existingMirror.AccessToken = req.AccessToken
		}
		return existingMirror, nil
	}
	if req.MirrorSourceID != 0 {
		mirrorSource, err := m.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get mirror source, err: %w, id: %d", err, req.MirrorSourceID)
		}
		mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	}

	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath

	mirror.RepositoryID = repo.ID
	mirror.Repository = repo

	mirror.Priority = req.Priority
	var metadataUpdate *database.MirrorRepoMetadataUpdate

	if !req.SkipSourcePath {
		sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceUrl)
		applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)

		metadataUpdate, err = m.prepareExistingMirrorRepoMetadataUpdate(ctx, repo, &mirror, nil, nil)
		if err != nil {
			return nil, err
		}
	}

	reqMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:       repo,
		CreateRepository: false,
		Metadata:         metadataUpdate,
		Mirror:           mirror,
		Urgent:           req.Urgent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror repo records: %w", err)
	}

	return reqMirror, nil
}

// MirrorFromSaas enqueues one workhub sync for an existing on-prem repository backed by a SaaS Git source.
func (m *mirrorComponentImpl) MirrorFromSaas(ctx context.Context, req types.MirrorFromSaasReq) (*types.MirrorFromSaasResponse, error) {
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	permission, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror sync permission: %w", err)
	}
	if !permission {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to sync this repository")
	}

	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if mirror != nil {
		task, err := m.requeueMirrorFromSaas(ctx, repo, mirror)
		if err != nil {
			return nil, err
		}
		return &types.MirrorFromSaasResponse{
			RepositoryID: repo.ID,
			MirrorID:     mirror.ID,
			TaskID:       task.ID,
			Status:       types.MirrorQueued,
		}, nil
	}

	syncVersion, err := m.syncVersionStore.FindByRepoTypeAndPath(ctx, repo.PathWithOutPrefix(), req.RepoType)
	if err != nil {
		return nil, fmt.Errorf("failed to find sync version, error: %w", err)
	}
	sourceURL := common.TrimPrefixCloneURLBySourceID(m.config.MultiSync.SaasSyncDomain, string(req.RepoType), req.Namespace, req.Name, syncVersion.SourceID)
	sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(sourceURL)
	applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	reqMirror := database.Mirror{
		SourceUrl:      sourceURL,
		RepositoryID:   repo.ID,
		Repository:     repo,
		SourceRepoPath: fmt.Sprintf("%s/%s", req.Namespace, req.Name),
		Priority:       types.MediumMirrorPriority,
	}
	createdMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:       repo,
		CreateRepository: false,
		Mirror:           reqMirror,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror repo records: %w", err)
	}
	return &types.MirrorFromSaasResponse{
		RepositoryID: repo.ID,
		MirrorID:     createdMirror.ID,
		TaskID:       createdMirror.CurrentTaskID,
		Status:       types.MirrorQueued,
	}, nil
}

// MirrorFromSaasStatus returns public progress for the current SaaS mirror task.
func (m *mirrorComponentImpl) MirrorFromSaasStatus(ctx context.Context, req types.MirrorFromSaasStatusReq) (*types.MirrorSyncStatusResponse, error) {
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	permission, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanRead)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror status permission: %w", err)
	}
	if !permission {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to read this repository")
	}

	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if mirror.CurrentTaskID == 0 || mirror.CurrentTask == nil ||
		mirror.CurrentTask.ID != mirror.CurrentTaskID || mirror.CurrentTask.MirrorID != mirror.ID {
		return nil, invalidMirrorTaskState(mirror, "current mirror task is missing or inconsistent")
	}

	result, jobID, err := classifyMirrorTask(repo.ID, mirror, req.RequestedTaskID)
	if err != nil {
		return nil, err
	}
	if result.Terminal {
		return result, nil
	}
	if jobID == 0 {
		return nil, invalidMirrorTaskState(mirror, "current mirror task job is missing")
	}
	return result, nil
}

// classifyMirrorTask maps persisted business status and returns the active workhub job ID.
func classifyMirrorTask(repositoryID int64, mirror *database.Mirror, requestedTaskID int64) (*types.MirrorSyncStatusResponse, int64, error) {
	task := mirror.CurrentTask
	result := &types.MirrorSyncStatusResponse{
		RepositoryID: repositoryID,
		MirrorID:     mirror.ID,
		TaskID:       task.ID,
		Status:       task.Status,
		Superseded:   requestedTaskID != 0 && requestedTaskID != task.ID,
		Progress:     task.Progress,
		UpdatedAt:    task.UpdatedAt,
	}

	switch task.Status {
	case types.MirrorQueued, types.MirrorRepoSyncStart, types.MirrorRepoSyncFailed:
		result.Phase = types.MirrorSyncPhaseRepo
		result.Retrying = task.Status == types.MirrorRepoSyncFailed
		return result, task.RepoJobID, nil
	case types.MirrorRepoSyncFatal:
		result.Phase = types.MirrorSyncPhaseRepo
		result.Terminal = true
		result.FailureReason = types.MirrorSyncFailureRepoSyncFailed
	case types.MirrorRepoSyncFinished, types.MirrorLfsSyncStart, types.MirrorLfsSyncFailed:
		result.Phase = types.MirrorSyncPhaseLFS
		result.RepoReady = true
		result.Retrying = task.Status == types.MirrorLfsSyncFailed
		return result, task.LFSJobID, nil
	case types.MirrorLfsSyncFinished:
		result.Phase = types.MirrorSyncPhaseDone
		result.RepoReady = true
		result.Terminal = true
	case types.MirrorLfsSyncFatal:
		result.Phase = types.MirrorSyncPhaseDone
		result.RepoReady = true
		result.Terminal = true
		result.FailureReason = types.MirrorSyncFailureLFSSyncFailed
	case types.MirrorLfsIncomplete:
		result.Phase = types.MirrorSyncPhaseDone
		result.RepoReady = true
		result.Terminal = true
		result.FailureReason = types.MirrorSyncFailureLFSIncomplete
	case types.MirrorRepoTooLarge:
		result.Phase = types.MirrorSyncPhaseDone
		result.RepoReady = true
		result.Terminal = true
		result.FailureReason = types.MirrorSyncFailureLFSTooLarge
	case types.MirrorCanceled:
		result.Phase = types.MirrorSyncPhaseDone
		result.RepoReady = task.AfterLastCommitID != ""
		result.Terminal = true
		result.FailureReason = types.MirrorSyncFailureCanceled
	default:
		return nil, 0, invalidMirrorTaskState(mirror, fmt.Sprintf("unsupported mirror task status %q", task.Status))
	}
	return result, 0, nil
}

// invalidMirrorTaskState returns a stable API error without exposing task error messages.
func invalidMirrorTaskState(mirror *database.Mirror, message string) error {
	publicErr := errorx.MirrorTaskStateInvalid(errors.New("mirror task state is invalid"), errorx.Ctx().
		Set("mirror_id", mirror.ID).
		Set("task_id", mirror.CurrentTaskID))
	return fmt.Errorf("%s: %w", message, publicErr)
}

// GetMirror returns mirror configuration and current task status for an existing repository.
func (m *mirrorComponentImpl) GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error) {
	var (
		status      types.MirrorTaskStatus
		progress    int8
		lastMessage string
	)
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	admin, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to get mirror for this repo")
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
	sourceURL, username, accessToken, err := normalizeMirrorSource(req.SourceUrl, req.Username, req.AccessToken)
	if err != nil {
		return nil, err
	}
	req.SourceUrl = sourceURL
	req.Username = username
	req.AccessToken = accessToken
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	admin, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to update mirror for this repo")
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}

	pushAccessToken, err := m.accessTokenStore.GetUserGitToken(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find access token, error: %w", err)
	}

	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.AccessToken = req.AccessToken
	mirror.PushUrl = req.PushUrl
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

// SyncMirror honors request authorization or existing user permissions before enqueuing a repository sync task.
func (m *mirrorComponentImpl) SyncMirror(ctx context.Context, req types.SyncMirrorReq) error {
	user, err := m.userStore.FindByUsername(ctx, req.CurrentUser, types.UserWithAdminPermissions)
	if err != nil {
		return errors.New("user does not exist")
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	if !(types.IsAdminAuthorized(ctx) || user.CanAdmin()) {
		canWrite, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanWrite)
		if err != nil {
			return fmt.Errorf("failed to check permission to sync mirror: %w", err)
		}

		if !canWrite {
			return errorx.ErrForbiddenMsg("write permission is required to sync mirror for this repo")
		}
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	_, err = m.requeueMirrorRepoTask(ctx, repo, mirror, nil, nil, mirror.Priority, req.Urgent)
	return err
}

// DeleteMirror validates access to an existing mirror repository and deletes it transactionally.
func (m *mirrorComponentImpl) DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error {
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	admin, err := m.repoComp.CheckUserRepoPermission(ctx, req.CurrentUser, repo, rebac.RepositoryCanAdmin)
	if err != nil {
		return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return fmt.Errorf("users do not have permission to delete mirror for this repo")
	}
	mirror, err := m.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if err := m.mirrorStore.DeleteWithTaskCancelTx(ctx, mirror.ID, m.mirrorJobClient); err != nil {
		return fmt.Errorf("failed to delete mirror, error: %w", err)
	}
	syncCache := ensureRepoSyncCache(ctx, &m.syncCacheMu, &m.syncCache, m.config)
	deleteRepoSyncCache(ctx, syncCache, repo.ID)
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
				TaskID:     m.CurrentTask.ID,
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
			ID:            mirror.ID,
			SourceUrl:     mirror.SourceUrl,
			Username:      mirror.Username,
			AccessToken:   mirror.AccessToken,
			LastUpdatedAt: mirror.LastUpdatedAt,
			LocalRepoPath: mirror.RepoPath(),
			LastMessage:   mirror.LastMessage,
			Status:        status,
			Progress:      mirror.Progress,
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

// inferMirrorRepositoryTypeFromPath classifies the provider path convention
// used by both new batch ingress and the legacy scheduler.
func inferMirrorRepositoryTypeFromPath(sourcePath string) types.RepositoryType {
	segments := strings.Split(strings.Trim(sourcePath, "/"), "/")
	if len(segments) >= 3 {
		switch strings.ToLower(segments[0]) {
		case "datasets":
			return types.DatasetRepo
		case "codes":
			return types.CodeRepo
		}
	}
	return types.ModelRepo
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
	deleteRepoSyncCache(ctx, syncCache, mirror.RepositoryID)
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
func deleteRepoSyncCache(ctx context.Context, syncCache mirrorcache.Cache, repoID int64) {
	if syncCache == nil || repoID == 0 {
		return
	}
	if err := syncCache.DeleteRepoSyncCache(ctx, repoID); err != nil {
		slog.WarnContext(ctx, "failed to delete mirror repo sync cache", slog.Any("error", err), slog.Int64("repo_id", repoID))
	}
}

// MirrorMetricsComponent reads snapshots and caches aggregate SQL queue counts.
type MirrorMetricsComponent struct {
	enabled    bool
	read       func(context.Context) (*cache.MirrorMetricsReadSnapshot, error)
	queues     func(context.Context) (types.MirrorMetricsQueues, error)
	saveQueues func(context.Context, *types.MirrorMetricsControl) error
}

// NewMirrorMetricsComponent creates readers without a background collector.
func NewMirrorMetricsComponent(cfg *config.Config) *MirrorMetricsComponent {
	result := &MirrorMetricsComponent{enabled: cfg.Mirror.Metrics.Enabled}
	if result.enabled {
		store := cache.NewMirrorMetricsStoreFromConfig(cfg)
		result.read, result.saveQueues = store.Read, store.PublishControl
		result.queues = database.NewMirrorStore().QueueCounts
	}
	return result
}

// Get loads queue counts only when their short Redis cache expires.
func (c *MirrorMetricsComponent) Get(ctx context.Context, q types.MirrorMetricsQuery) (*types.MirrorMetricsResponse, error) {
	if !c.enabled {
		result := metricsResponse(time.Now().UnixMilli(), q)
		result.Status = "disabled"
		return result, nil
	}
	snapshot, err := c.read(ctx)
	if err != nil {
		return nil, err
	}
	if snapshot.Control == nil {
		c.loadQueues(ctx, snapshot)
	}
	return aggregateMirrorMetrics(snapshot, q), nil
}

// loadQueues tolerates duplicate queries and cache overwrites across API replicas.
func (c *MirrorMetricsComponent) loadQueues(ctx context.Context, snapshot *cache.MirrorMetricsReadSnapshot) {
	started := time.Now()
	queryCtx, cancel := context.WithTimeout(ctx, time.Second)
	queues, err := c.queues(queryCtx)
	cancel()
	if err != nil {
		snapshot.Partial = true
		return
	}
	snapshot.Control = &types.MirrorMetricsControl{SchemaVersion: cache.MirrorMetricsSchemaVersion, SampledAtMS: started.UnixMilli(), Queues: queues}
	writeCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err := c.saveQueues(writeCtx, snapshot.Control); err != nil {
		snapshot.Partial = true
	}
}

// metricsResponse initializes arrays and the current-snapshot expiration limits.
func metricsResponse(now int64, q types.MirrorMetricsQuery) *types.MirrorMetricsResponse {
	return &types.MirrorMetricsResponse{SchemaVersion: cache.MirrorMetricsSchemaVersion, ServerTimeMS: now, RefreshAfterMS: 1000, Status: "ok",
		FreshnessLimitsMS: types.MirrorMetricsFreshnessLimits{Snapshot: mirrorMetricsSnapshotFreshness.Milliseconds(), Speed: mirrorMetricsSpeedFreshness.Milliseconds(), Queue: cache.MirrorMetricsQueueRetention.Milliseconds()},
		LFSWorkers:        []types.MirrorMetricsLFSWorker{},
		Instances:         types.MirrorMetricsInstancePage{Page: q.InstancePage, Per: q.InstancePer, Items: []types.MirrorMetricsInstance{}}, Tasks: types.MirrorMetricsTaskPage{Page: q.Page, Per: q.Per, Items: []types.MirrorMetricsTask{}}}
}

// aggregateMirrorMetrics builds global totals before filtering the display tables.
func aggregateMirrorMetrics(snapshot *cache.MirrorMetricsReadSnapshot, q types.MirrorMetricsQuery) *types.MirrorMetricsResponse {
	response := metricsResponse(snapshot.NowMS, q)
	until := snapshot.NowMS + mirrorMetricsSnapshotFreshness.Milliseconds()
	overview := &types.MirrorMetricsOverview{Freshness: "unavailable", CountsValidUntilMS: types.MirrorMetricsStageDeadlines{Repo: until, LFS: until}, OnlineInstances: &types.MirrorMetricsStageCounts{}, BusyInstances: &types.MirrorMetricsStageCounts{}, ExecutingTasks: &types.MirrorMetricsStageCounts{}}
	response.Overview = overview
	if snapshot.Control != nil {
		overview.SampledAtMS = metricsPointer(snapshot.Control.SampledAtMS)
		overview.Queues = &snapshot.Control.Queues
		overview.Freshness = metricsFreshness(snapshot.NowMS, overview.SampledAtMS, cache.MirrorMetricsQueueRetention.Milliseconds())
	}
	instances := make([]types.MirrorMetricsInstance, 0, len(snapshot.Instances))
	tasks := make([]types.MirrorMetricsTask, 0)
	for _, raw := range snapshot.Instances {
		instance, current := metricsWorkerSnapshot(raw, snapshot.NowMS)
		if instance.Service == "lfs" {
			response.LFSWorkers = append(response.LFSWorkers, types.MirrorMetricsLFSWorker{
				InstanceID: instance.InstanceID, Name: instance.Name, RuntimeState: instance.RuntimeState,
				SampledAtMS: instance.SampledAtMS, DownloadBPS: instance.DownloadBPS, UploadBPS: instance.UploadBPS,
			})
		}
		if len(current) > 0 {
			metricsIncrement(overview.BusyInstances, instance.Service)
		}
		if instance.Freshness == "fresh" && (instance.RuntimeState == "ready" || instance.RuntimeState == "stopping") {
			metricsCountWorker(overview, instance, snapshot.NowMS)
		}
		for _, task := range current {
			metricsIncrement(overview.ExecutingTasks, task.Stage)
			if metricsInstanceSelected(q, instance) && (q.TaskID == "" || q.TaskID == task.TaskID) {
				tasks = append(tasks, task)
			}
		}
		instances = append(instances, instance)
	}
	overview.Throughput = metricsThroughput(instances, snapshot.NowMS)
	selected := make([]types.MirrorMetricsInstance, 0, len(instances))
	for _, instance := range instances {
		if metricsInstanceSelected(q, instance) {
			selected = append(selected, instance)
		}
	}
	sort.Slice(response.LFSWorkers, func(i, j int) bool { return response.LFSWorkers[i].InstanceID < response.LFSWorkers[j].InstanceID })
	sort.Slice(selected, func(i, j int) bool { return selected[i].InstanceID < selected[j].InstanceID })
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].TaskID == tasks[j].TaskID {
			return *tasks[i].InstanceID < *tasks[j].InstanceID
		}
		return tasks[i].TaskID < tasks[j].TaskID
	})
	response.Instances.Total, response.Tasks.Total = len(selected), len(tasks)
	response.Instances.Items = metricsPage(selected, q.InstancePage, q.InstancePer)
	response.Tasks.Items = metricsPage(tasks, q.Page, q.Per)
	if snapshot.Partial || snapshot.Control == nil {
		response.Status = "partial"
	}
	return response
}

// metricsWorkerSnapshot uses one sample timestamp for liveness and execution state.
func metricsWorkerSnapshot(raw types.MirrorMetricsSnapshot, now int64) (types.MirrorMetricsInstance, []types.MirrorMetricsTask) {
	instance := raw.Instance
	instance.Freshness = metricsFreshness(now, instance.SampledAtMS, mirrorMetricsSnapshotFreshness.Milliseconds())
	live := instance.Freshness == "fresh" && (instance.RuntimeState == "ready" || instance.RuntimeState == "stopping")
	speedFresh := live && instance.Service == "lfs" && metricsFreshness(now, instance.SampledAtMS, mirrorMetricsSpeedFreshness.Milliseconds()) == "fresh"
	if !speedFresh {
		instance.DownloadBPS = nil
		instance.UploadBPS = nil
	}
	instance.ExecutingTasks = 0
	tasks := make([]types.MirrorMetricsTask, 0)
	if !live {
		return instance, tasks
	}
	for _, task := range raw.Executions {
		task.InstanceID = metricsPointer(instance.InstanceID)
		task.Stage = instance.Service
		task.SampledAtMS = instance.SampledAtMS
		task.Freshness = "fresh"
		if !speedFresh {
			task.DownloadBPS = nil
			task.UploadBPS = nil
		}
		tasks = append(tasks, task)
	}
	instance.ExecutingTasks = len(tasks)
	return instance, tasks
}

// metricsCountWorker counts presence and bounds each service by its oldest snapshot.
func metricsCountWorker(overview *types.MirrorMetricsOverview, instance types.MirrorMetricsInstance, now int64) {
	if instance.RuntimeState == "ready" {
		metricsIncrement(overview.OnlineInstances, instance.Service)
	}
	until := min(now, *instance.SampledAtMS) + mirrorMetricsSnapshotFreshness.Milliseconds()
	if instance.Service == "repo" {
		overview.CountsValidUntilMS.Repo = min(overview.CountsValidUntilMS.Repo, until)
	}
	if instance.Service == "lfs" {
		overview.CountsValidUntilMS.LFS = min(overview.CountsValidUntilMS.LFS, until)
	}
}

// metricsThroughput sums each direction only when all participating LFS rates are known.
// An empty set keeps observed zero totals; unknown contributions remain unknown.
func metricsThroughput(instances []types.MirrorMetricsInstance, now int64) *types.MirrorMetricsThroughput {
	result := &types.MirrorMetricsThroughput{SampledAtMS: metricsPointer(now),
		ClusterDownloadBPS: metricsPointer(0.0), ClusterUploadBPS: metricsPointer(0.0)}
	for _, instance := range instances {
		if instance.Service != "lfs" || instance.Freshness != "fresh" || (instance.RuntimeState != "ready" && instance.RuntimeState != "stopping") {
			continue
		}
		result.ClusterDownloadBPS = metricsAddRate(result.ClusterDownloadBPS, instance.DownloadBPS)
		result.ClusterUploadBPS = metricsAddRate(result.ClusterUploadBPS, instance.UploadBPS)
		if instance.SampledAtMS != nil && *instance.SampledAtMS < *result.SampledAtMS {
			result.SampledAtMS = instance.SampledAtMS
		}
	}
	return result
}

// metricsInstanceSelected applies display filters without changing aggregate totals.
func metricsInstanceSelected(q types.MirrorMetricsQuery, instance types.MirrorMetricsInstance) bool {
	return (q.Service == "all" || q.Service == instance.Service) && (q.InstanceID == "" || q.InstanceID == instance.InstanceID)
}

// metricsFreshness applies an age bound without calibrating worker clocks.
func metricsFreshness(now int64, at *int64, limit int64) string {
	if at == nil {
		return "unavailable"
	}
	if now-*at > limit || *at > now+1000 {
		return "stale"
	}
	return "fresh"
}

// metricsAddRate propagates unknown or invalid contributions without reviving an unknown sum.
func metricsAddRate(sum, value *float64) *float64 {
	if sum == nil || value == nil || *value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return nil
	}
	return metricsPointer(*sum + *value)
}

// metricsIncrement updates the matching service count only.
func metricsIncrement(counts *types.MirrorMetricsStageCounts, stage string) {
	if stage == "repo" {
		counts.Repo++
	}
	if stage == "lfs" {
		counts.LFS++
	}
}

// metricsPage avoids overflowing integer multiplication for attacker-supplied pages.
func metricsPage[T any](items []T, page, per int) []T {
	if per <= 0 || page <= 0 || len(items) == 0 || page-1 > len(items)/per {
		return []T{}
	}
	start := (page - 1) * per
	return items[start:min(start+per, len(items))]
}

// metricsPointer preserves nullable scalar values in responses.
func metricsPointer[T any](value T) *T { return &value }

const (
	// mirrorMetricsSampleInterval is shared by Repo and LFS workers.
	mirrorMetricsSampleInterval = time.Second
	// mirrorMetricsSnapshotFreshness bounds worker presence and current execution state.
	mirrorMetricsSnapshotFreshness = 10 * time.Second
	// mirrorMetricsSpeedFreshness hides outdated rates sooner than worker presence.
	mirrorMetricsSpeedFreshness = 3 * time.Second
	// mirrorMetricsPublishTimeout keeps Redis failure outside business execution.
	mirrorMetricsPublishTimeout = 200 * time.Millisecond
)

// mirrorMetricsExecutionKey attaches byte accounting to an existing work context.
type mirrorMetricsExecutionKey struct{}

// mirrorMetricsExecution holds only current work, byte counters and verified LFS progress.
type mirrorMetricsExecution struct {
	reporter              *MirrorMetricsReporter
	task                  types.MirrorMetricsTask
	bytes                 atomic.Int64
	uploadedBytes         atomic.Int64
	previousBytes         int64
	previousUploadedBytes int64
	previousSample        time.Time
	completeBytes         int64
	partBytes             int64
	objects               map[string]int64
	complete              map[string]bool
	totalBytes            int64
	filesDone             int
	filesTotal            int
	progressKnown         bool
}

// MirrorMetricsReporter maintains in-memory observations and one bounded timer loop.
type MirrorMetricsReporter struct {
	mu                    sync.Mutex
	store                 *cache.MirrorMetricsStore
	instance              types.MirrorMetricsInstance
	executions            map[*mirrorMetricsExecution]bool
	totalBytes            atomic.Int64
	totalUploadedBytes    atomic.Int64
	previousBytes         int64
	previousUploadedBytes int64
	lastSample            time.Time
	stop                  chan struct{}
	done                  chan struct{}
	stopOnce              sync.Once
}

// NewMirrorMetricsReporter starts sampling; disabled metrics leave work unchanged.
func NewMirrorMetricsReporter(cfg *config.Config, service string) *MirrorMetricsReporter {
	if !cfg.Mirror.Metrics.Enabled {
		return nil
	}
	return newMirrorMetricsReporter(cache.NewMirrorMetricsStoreFromConfig(cfg), cfg, service)
}

// newMirrorMetricsReporter accepts a Redis adapter for focused integration tests.
func newMirrorMetricsReporter(store *cache.MirrorMetricsStore, cfg *config.Config, service string) *MirrorMetricsReporter {
	now := time.Now()
	normal := max(cfg.Mirror.WorkerNumber, 1)
	workerConfig := types.MirrorMetricsWorkerConfig{WorkerNumber: cfg.Mirror.WorkerNumber, NormalWorkers: normal, UrgentWorkers: workhub.UrgentMaxWorkers(normal)}
	if service == "lfs" {
		workerConfig.LFSConcurrency = metricsPointer(cfg.Mirror.LfsConcurrency)
		workerConfig.PartSizeBytes = metricsPointer(strconv.FormatInt(int64(cfg.Mirror.PartSize)*1024*1024, 10))
	}
	r := &MirrorMetricsReporter{store: store, instance: types.MirrorMetricsInstance{InstanceID: uuid.NewString(), Name: cfg.UniqueServiceName, Service: service, StartedAtMS: now.UnixMilli(), RuntimeState: "starting", Config: workerConfig}, executions: make(map[*mirrorMetricsExecution]bool), lastSample: now, stop: make(chan struct{}), done: make(chan struct{})}
	go r.run()
	return r
}

// SetState copies the existing process lifecycle; publication waits for the timer.
func (r *MirrorMetricsReporter) SetState(state string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.instance.RuntimeState = state
	r.mu.Unlock()
}

// Stop requests one final stopped snapshot and waits within the shutdown budget.
func (r *MirrorMetricsReporter) Stop(ctx context.Context) {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stop) })
	select {
	case <-r.done:
	case <-ctx.Done():
	}
}

// Track attaches instrumentation without publishing a claimed or queued task.
// MirrorMetricsRunning adds it only after the worker's ordinary execution checks succeed.
func (r *MirrorMetricsReporter) Track(ctx context.Context, task types.MirrorMetricsTask) (context.Context, func()) {
	if r == nil {
		return ctx, func() {}
	}
	task.InstanceID = metricsPointer(r.instance.InstanceID)
	entry := &mirrorMetricsExecution{reporter: r, task: task, objects: make(map[string]int64), complete: make(map[string]bool), previousSample: time.Now()}
	var once sync.Once
	release := func() { once.Do(func() { r.mu.Lock(); delete(r.executions, entry); r.mu.Unlock() }) }
	return context.WithValue(ctx, mirrorMetricsExecutionKey{}, entry), release
}

// MirrorMetricsRunning records the already validated work without maintaining a second task state machine.
func MirrorMetricsRunning(ctx context.Context, task types.MirrorMetricsTask) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil {
		return
	}
	r := entry.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.task.RepoPath = task.RepoPath
	entry.task.StartedAtMS = task.StartedAtMS
	now := time.Now()
	entry.task.ExecutionStartedAtMS = metricsPointer(now.UnixMilli())
	entry.previousSample = now
	entry.task.Operation = metricsPointer("prepare")
	r.executions[entry] = true
}

// MirrorMetricsOperation copies a display label for the current work phase.
func MirrorMetricsOperation(ctx context.Context, operation string) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil {
		return
	}
	entry.reporter.mu.Lock()
	entry.task.Operation = metricsPointer(operation)
	entry.reporter.mu.Unlock()
}

// MirrorMetricsWrapBody counts delivered upstream bytes, including reads returning bytes together with an error.
func MirrorMetricsWrapBody(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil || body == nil {
		return body
	}
	return &mirrorMetricsDownloadBody{ReadCloser: body, execution: entry}
}

// mirrorMetricsDownloadBody preserves the wrapped stream's error and close semantics.
type mirrorMetricsDownloadBody struct {
	io.ReadCloser
	execution *mirrorMetricsExecution
}

// Read does no allocation, Redis work, or mutex acquisition on the transfer hot path.
func (b *mirrorMetricsDownloadBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.execution.bytes.Add(int64(n))
		b.execution.reporter.totalBytes.Add(int64(n))
	}
	return n, err
}

// MirrorMetricsInitProgress records totals only after object discovery and storage verification complete.
func MirrorMetricsInitProgress(ctx context.Context, total, complete int64, filesTotal, filesDone int) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil {
		return
	}
	r := entry.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.totalBytes = max(total, 0)
	entry.completeBytes = min(max(complete, 0), entry.totalBytes)
	entry.filesTotal = filesTotal
	entry.filesDone = filesDone
	entry.progressKnown = true
}

// MirrorMetricsAddPart credits bounded, confirmed uploaded bytes without counting network retries twice.
func MirrorMetricsAddPart(ctx context.Context, oid string, size, delta int64) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil || delta <= 0 || size <= 0 {
		return
	}
	r := entry.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.complete[oid] {
		return
	}
	amount := min(delta, size-entry.objects[oid])
	if amount <= 0 {
		return
	}
	entry.objects[oid] += amount
	entry.partBytes += amount
}

// MirrorMetricsCompleteObject moves confirmed parts into the verified whole-object total exactly once.
func MirrorMetricsCompleteObject(ctx context.Context, oid string, size int64) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil || size < 0 {
		return
	}
	r := entry.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.complete[oid] {
		return
	}
	entry.complete[oid] = true
	entry.partBytes -= entry.objects[oid]
	delete(entry.objects, oid)
	entry.completeBytes = min(entry.totalBytes, entry.completeBytes+size)
	entry.filesDone = min(entry.filesTotal, entry.filesDone+1)
}

// MirrorMetricsResetObject removes parts invalidated by a failed final object verification.
func MirrorMetricsResetObject(ctx context.Context, oid string) {
	entry, _ := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil {
		return
	}
	r := entry.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.partBytes -= entry.objects[oid]
	delete(entry.objects, oid)
}

// run samples and writes once per second; failed writes are replaced on the next tick.
func (r *MirrorMetricsReporter) run() {
	defer close(r.done)
	defer r.store.Close()
	ticker := time.NewTicker(mirrorMetricsSampleInterval)
	defer ticker.Stop()
	var lastLog time.Time
	for {
		stopping := false
		select {
		case <-ticker.C:
		case <-r.stop:
			stopping = true
		}
		if stopping {
			r.SetState("stopped")
		}
		snapshot := r.capture(time.Now())
		ctx, cancel := context.WithTimeout(context.Background(), mirrorMetricsPublishTimeout)
		err := r.store.PublishSnapshot(ctx, snapshot)
		cancel()
		if err != nil && time.Since(lastLog) > 30*time.Second {
			slog.Warn("mirror metrics snapshot unavailable", slog.String("service", r.instance.Service), slog.Any("error", err))
			lastLog = time.Now()
		}
		if stopping {
			return
		}
	}
}

// capture copies current work under the mutex and computes byte deltas using monotonic time.
func (r *MirrorMetricsReporter) capture(now time.Time) types.MirrorMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instance.SampledAtMS = metricsPointer(now.UnixMilli())
	r.instance.ExecutingTasks = len(r.executions)
	duration := now.Sub(r.lastSample).Seconds()
	if r.instance.Service == "lfs" && duration > 0 {
		down, up := r.totalBytes.Load(), r.totalUploadedBytes.Load()
		r.instance.DownloadBPS = metricsPointer(float64(down-r.previousBytes) / duration)
		r.instance.UploadBPS = metricsPointer(float64(up-r.previousUploadedBytes) / duration)
		r.instance.DownloadedBytes = metricsPointer(strconv.FormatInt(down, 10))
		r.instance.UploadedBytes = metricsPointer(strconv.FormatInt(up, 10))
		r.previousBytes, r.previousUploadedBytes = down, up
	}
	tasks := make([]types.MirrorMetricsTask, 0, len(r.executions))
	for entry := range r.executions {
		tasks = append(tasks, r.captureExecution(entry, now))
	}
	r.lastSample = now
	if r.instance.RuntimeState == "stopped" {
		tasks = []types.MirrorMetricsTask{}
		r.instance.ExecutingTasks = 0
	}
	return types.MirrorMetricsSnapshot{SchemaVersion: cache.MirrorMetricsSchemaVersion, Instance: r.instance, Executions: tasks}
}

// captureExecution copies only currently running work and its LFS measurements.
func (r *MirrorMetricsReporter) captureExecution(entry *mirrorMetricsExecution, now time.Time) types.MirrorMetricsTask {
	task := entry.task
	if task.Stage != "lfs" {
		return task
	}
	down, up := entry.bytes.Load(), entry.uploadedBytes.Load()
	task.DownloadedBytes, task.UploadedBytes = metricsPointer(strconv.FormatInt(down, 10)), metricsPointer(strconv.FormatInt(up, 10))
	if seconds := now.Sub(entry.previousSample).Seconds(); seconds > 0 {
		task.DownloadBPS = metricsPointer(float64(down-entry.previousBytes) / seconds)
		task.UploadBPS = metricsPointer(float64(up-entry.previousUploadedBytes) / seconds)
	}
	entry.previousBytes, entry.previousUploadedBytes, entry.previousSample = down, up, now
	if entry.progressKnown {
		task.LFS = &types.MirrorMetricsLFS{TotalBytes: metricsPointer(strconv.FormatInt(entry.totalBytes, 10)), CompleteBytes: metricsPointer(strconv.FormatInt(entry.completeBytes, 10)), ConfirmedPartBytes: metricsPointer(strconv.FormatInt(entry.partBytes, 10)), FilesDone: metricsPointer(entry.filesDone), FilesTotal: metricsPointer(entry.filesTotal)}
		if entry.totalBytes > 0 {
			ceiling := 100.0
			if entry.filesDone < entry.filesTotal {
				ceiling = 99
			}
			task.LFS.ProgressPercent = metricsPointer(min(ceiling, float64(entry.completeBytes+entry.partBytes)/float64(entry.totalBytes)*100))
		}
	}
	return task
}

// MirrorMetricsUploadTransport observes S3 PUT request bodies after SDK buffering and hashing.
// It counts bytes consumed by HTTP transport, including failed attempts and
// retries, rather than acknowledged object size or local buffer reads. Headers,
// TLS framing and TCP retransmissions are outside this application-level metric.
func MirrorMetricsUploadTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &mirrorMetricsUploadTransport{base: base}
}

// mirrorMetricsUploadTransport instruments only LFS execution object uploads.
type mirrorMetricsUploadTransport struct{ base http.RoundTripper }

// RoundTrip preserves request ownership and instruments replayed request bodies.
func (t *mirrorMetricsUploadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	entry, _ := request.Context().Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	if entry == nil || entry.task.Stage != "lfs" || request.Method != http.MethodPut || request.Body == nil || request.Header.Get("X-Amz-Copy-Source") != "" {
		return t.base.RoundTrip(request)
	}
	cloned := request.Clone(request.Context())
	cloned.Body = &mirrorMetricsUploadBody{ReadCloser: request.Body, execution: entry}
	if request.GetBody != nil {
		cloned.GetBody = func() (io.ReadCloser, error) {
			body, err := request.GetBody()
			if err != nil {
				return nil, err
			}
			return &mirrorMetricsUploadBody{ReadCloser: body, execution: entry}, nil
		}
	}
	return t.base.RoundTrip(cloned)
}

// mirrorMetricsUploadBody keeps upstream download accounting independent from outbound reads.
type mirrorMetricsUploadBody struct {
	io.ReadCloser
	execution *mirrorMetricsExecution
}

// Read records partial failed sends without blocking the transfer on Redis or locks.
func (b *mirrorMetricsUploadBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.execution.uploadedBytes.Add(int64(n))
		b.execution.reporter.totalUploadedBytes.Add(int64(n))
	}
	return n, err
}
