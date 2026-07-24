package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
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
	BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error
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
	mirrorRepoJobClient := workhub.NewMirrorRepoJobClient(jobClient, workhub.MirrorJobClientConfig{MaxRetryCount: config.Mirror.MaxRetryCount})
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

// mapNamespaceAndName resolves a remote namespace to a lowercase local namespace.
func (m *mirrorComponentImpl) mapNamespaceAndName(sourceNamespace string) string {
	n, err := m.mirrorNamespaceMappingStore.FindBySourceNamespace(context.Background(), sourceNamespace)
	if err != nil {
		return "aiwizards"
	}
	if n.TargetNamespace == "" {
		return "aiwizards"
	}
	return strings.ToLower(strings.TrimSpace(n.TargetNamespace))
}

// CreateMirror creates a mirror configuration for an existing repository.
func (m *mirrorComponentImpl) CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error) {
	sourceURL, username, accessToken, err := normalizeMirrorSource(req.SourceUrl, req.Username, req.AccessToken)
	if err != nil {
		return nil, err
	}
	req.SourceUrl = sourceURL
	req.Username = username
	req.AccessToken = accessToken
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

	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath

	mirror.RepositoryID = repo.ID
	mirror.Repository = repo

	mirror.Priority = types.LowMirrorPriority

	sourceType, sourcePath, _ := common.GetSourceTypeAndPathFromURL(req.SourceUrl)
	applyMirrorRepositorySourcePath(repo, sourceType, sourcePath)
	reqMirror, err := m.mirrorRepoStore.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository:       repo,
		CreateRepository: false,
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
	permission, err := m.repoComp.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror sync permission: %w", err)
	}
	if !permission.CanWrite {
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
	permission, err := m.repoComp.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror status permission: %w", err)
	}
	if !permission.CanRead {
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
	sourceURL, username, accessToken, err := normalizeMirrorSource(req.SourceUrl, req.Username, req.AccessToken)
	if err != nil {
		return nil, err
	}
	req.SourceUrl = sourceURL
	req.Username = username
	req.AccessToken = accessToken
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

// SyncMirror validates access to an existing mirror repository and enqueues a new repo sync task.
func (m *mirrorComponentImpl) SyncMirror(ctx context.Context, req types.SyncMirrorReq) error {
	user, err := m.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		canWrite, err := m.repoComp.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleWrite)
		if err != nil {
			return fmt.Errorf("failed to check permission to sync mirror: %w", err)
		}

		if !canWrite {
			return errorx.ErrForbiddenMsg("write permission is required to sync mirror for this repo")
		}
	}
	repo, err := m.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
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

// BatchCreate normalizes mirror sources and updates existing mirrors with provided credentials.
func (m *mirrorComponentImpl) BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error {
	var (
		toBeUpdatedMirrors []database.Mirror
		toBeCreatedMirrors []database.Mirror
		sourceURLs         []string
		existingSourceURLs []string
	)
	sourceURLMirrorMapping := make(map[string]types.MirrorReq)
	for i, mirror := range req.Mirrors {
		sourceURL, username, accessToken, err := normalizeMirrorSource(
			mirror.SourceURL, mirror.Username, mirror.AccessToken,
		)
		if err != nil {
			return fmt.Errorf("mirror index %d: %w", i, err)
		}
		mirror.SourceURL = sourceURL
		mirror.Username = username
		mirror.AccessToken = accessToken
		req.Mirrors[i] = mirror
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
				mirror.Priority = int8(types.LowMirrorPriority)
			}
			eMirror.Priority = types.MirrorPriority(mirror.Priority)
			eMirror.RemoteUpdatedAt = mirror.UpdatedAt
			if mirror.Username != "" {
				eMirror.Username = mirror.Username
				eMirror.AccessToken = mirror.AccessToken
			}
			toBeUpdatedMirrors = append(toBeUpdatedMirrors, eMirror)
		}
		existingSourceURLs = append(existingSourceURLs, eMirror.SourceUrl)
	}
	for _, mirror := range req.Mirrors {
		if mirror.Priority == 0 {
			mirror.Priority = int8(types.LowMirrorPriority)
		}
		if !slices.Contains(existingSourceURLs, mirror.SourceURL) {
			dbMirror := database.Mirror{
				SourceUrl:       mirror.SourceURL,
				Username:        mirror.Username,
				AccessToken:     mirror.AccessToken,
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
