package lfssyncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/mirror/hook"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

type (
	repoPathKey          string
	sourceUrlKey         string
	defaultBranchKey     string
	sourceUsernameKey    string
	sourceAccessTokenKey string
	lfsLoggerKey         struct{}
)

// unexpectedHTTPStatusError preserves a rejected response status after its body is closed.
type unexpectedHTTPStatusError struct {
	statusCode int
}

// Error returns the rejected HTTP status.
func (e *unexpectedHTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status code %d", e.statusCode)
}

var (
	rk            repoPathKey          = "repoPath"
	suk           sourceUrlKey         = "sourceUrl"
	dbk           defaultBranchKey     = "defaultBranch"
	sunk          sourceUsernameKey    = "sourceUsername"
	satk          sourceAccessTokenKey = "sourceAccessToken"
	maxRetries                         = 3
	maxGroupCount                      = 15
	maxPartNum                         = 1000
	maxGroupSize  int64                = 10 * 1024 * 1024 * 1024 // 10GB
)

// LfsSyncWorker synchronizes Git LFS objects for mirror tasks.
type LfsSyncWorker struct {
	mirrorTaskStore    database.MirrorTaskStore
	lfsMetaObjectStore database.LfsMetaObjectStore
	ossClient          s3.Client
	ossCore            s3.Core
	config             *config.Config
	syncCache          cache.Cache
	mu                 sync.Mutex
	httpClient         *http.Client
	msgSender          hook.MessageSender
	git                gitserver.GitServer
	workflowClient     temporal.Client
}

// mirrorTaskByteProgress persists one task's LFS completion by uploaded bytes.
// Object-level accounting prevents resumed or re-uploaded parts from being counted twice.
type mirrorTaskByteProgress struct {
	mu                sync.Mutex
	task              *database.MirrorTask
	store             database.MirrorTaskStore
	totalBytes        int64
	uploadedBytes     int64
	objectBytes       map[string]int64
	lastPersistedRate int
}

// newMirrorTaskByteProgress initializes task progress from objects that already exist in storage.
func newMirrorTaskByteProgress(
	task *database.MirrorTask,
	store database.MirrorTaskStore,
	totalBytes, uploadedBytes int64,
) *mirrorTaskByteProgress {
	if totalBytes < 0 {
		totalBytes = 0
	}
	if uploadedBytes < 0 {
		uploadedBytes = 0
	}
	if uploadedBytes > totalBytes {
		uploadedBytes = totalBytes
	}
	lastPersistedRate := 0
	if task != nil {
		lastPersistedRate = task.Progress
	}
	return &mirrorTaskByteProgress{
		task:              task,
		store:             store,
		totalBytes:        totalBytes,
		uploadedBytes:     uploadedBytes,
		objectBytes:       make(map[string]int64),
		lastPersistedRate: lastPersistedRate,
	}
}

// persistInitial records progress contributed by complete objects found before transfer starts.
func (p *mirrorTaskByteProgress) persistInitial(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.persistLocked(ctx)
}

// addObjectBytes credits newly uploaded bytes for one incomplete LFS object.
func (p *mirrorTaskByteProgress) addObjectBytes(ctx context.Context, oid string, objectSize, uploadedBytes int64) error {
	if p == nil || uploadedBytes <= 0 || objectSize <= 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	credited := p.objectBytes[oid]
	remaining := objectSize - credited
	if remaining <= 0 {
		return nil
	}
	if uploadedBytes > remaining {
		uploadedBytes = remaining
	}
	p.objectBytes[oid] = credited + uploadedBytes
	p.uploadedBytes += uploadedBytes
	if p.uploadedBytes > p.totalBytes {
		p.uploadedBytes = p.totalBytes
	}
	return p.persistLocked(ctx)
}

// completeObject credits any bytes not observed through multipart callbacks after object verification succeeds.
func (p *mirrorTaskByteProgress) completeObject(ctx context.Context, oid string, objectSize int64) error {
	if p == nil || objectSize <= 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	remaining := objectSize - p.objectBytes[oid]
	if remaining <= 0 {
		return nil
	}
	p.objectBytes[oid] = objectSize
	p.uploadedBytes += remaining
	if p.uploadedBytes > p.totalBytes {
		p.uploadedBytes = p.totalBytes
	}
	return p.persistLocked(ctx)
}

// persistLocked stores only increasing integer percentages and reserves 100 for final task success.
func (p *mirrorTaskByteProgress) persistLocked(ctx context.Context) error {
	if p.task == nil || p.store == nil || p.totalBytes <= 0 {
		return nil
	}
	rate := int(math.Floor(float64(p.uploadedBytes) / float64(p.totalBytes) * 100))
	if rate >= 100 {
		rate = 99
	}
	if rate <= p.lastPersistedRate {
		return nil
	}
	p.task.Progress = rate
	if _, err := p.store.UpdateProgress(ctx, *p.task); err != nil {
		return fmt.Errorf("failed to update mirror task progress: %w", err)
	}
	p.lastPersistedRate = rate
	return nil
}

// NewLfsSyncWorker creates an LFS synchronization worker.
func NewLfsSyncWorker(config *config.Config) (*LfsSyncWorker, error) {
	if config.Mirror.PartSize <= 0 {
		return nil, fmt.Errorf("LFS multipart part size must be positive: %d", config.Mirror.PartSize)
	}
	var err error
	w := &LfsSyncWorker{}
	w.config = config
	w.mirrorTaskStore = database.NewMirrorTaskStore()
	w.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	w.config = config
	w.ossClient, err = s3.NewMinio(config)
	if err != nil {
		return nil, fmt.Errorf("initializing minio: %w", err)
	}

	w.ossCore, err = s3.NewMinioCore(config)
	if err != nil {
		return nil, fmt.Errorf("initializing minio core: %w", err)
	}

	cache, err := cache.NewCache(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	w.syncCache = cache

	if !config.Proxy.Enable || config.Proxy.URL == "" {
		w.httpClient = &http.Client{}
	} else {
		proxyURL, err := url.Parse(config.Proxy.URL)
		if err != nil {
			return nil, fmt.Errorf("fail to parse proxy url: %w", err)
		}
		w.httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	}

	msgSender := hook.NewMessageSender(
		fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken),
		rpc.WithJSONHeader(),
	)
	w.msgSender = msgSender

	w.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	w.workflowClient = temporal.GetClient()

	return w, nil
}

// SyncLFS refreshes LFS metadata for the synced commit, then downloads missing
// objects and publishes the repository head.
func (w *LfsSyncWorker) SyncLFS(ctx context.Context, mt *database.MirrorTask) error {
	ctx = w.withLFSContext(ctx, mt)
	if err := w.refreshLfsMetaObjects(ctx, mt); err != nil {
		return err
	}
	return w.SyncLfs(ctx, mt)
}

// refreshLfsMetaObjects scans the repository at the repo-sync result commit and
// atomically replaces local LFS metadata for this repository.
func (w *LfsSyncWorker) refreshLfsMetaObjects(ctx context.Context, mt *database.MirrorTask) error {
	if mt == nil || mt.Mirror == nil || mt.Mirror.Repository == nil {
		return fmt.Errorf("invalid mirror task")
	}
	if mt.AfterLastCommitID == "" {
		return fmt.Errorf("mirror task %d has empty after commit id", mt.ID)
	}

	repo := mt.Mirror.Repository
	namespace, name, err := common.GetNamespaceAndNameFromPath(repo.Path)
	if err != nil {
		return fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
	}
	relativePath := repo.GitalyPath()
	lfsPointers, err := w.git.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace:    namespace,
		Name:         name,
		Ref:          mt.AfterLastCommitID,
		RepoType:     repo.RepositoryType,
		RelativePath: relativePath,
	})
	if err != nil {
		return fmt.Errorf("failed to get all lfs pointers: %w", err)
	}

	lfsMetaObjects, totalSize := lfsPointersToMetaObjects(repo.ID, lfsPointers)
	repo.LFSObjectsSize = totalSize
	if err := w.lfsMetaObjectStore.BulkUpdateOrCreate(ctx, repo.ID, lfsMetaObjects); err != nil {
		return fmt.Errorf("failed to bulk update or create lfs meta objects: %w", err)
	}

	loggerFromLFSContext(ctx).InfoContext(ctx, "refreshed lfs meta objects",
		slog.String("afterCommitID", mt.AfterLastCommitID),
		slog.Int("lfsCount", len(lfsMetaObjects)),
		slog.Int64("lfsObjectsSize", totalSize),
	)
	return nil
}

// withLFSContext attaches task logging fields and legacy values required by the LFS sync code.
func (w *LfsSyncWorker) withLFSContext(ctx context.Context, mt *database.MirrorTask) context.Context {
	if mt == nil || mt.Mirror == nil || mt.Mirror.Repository == nil {
		return ctx
	}
	repo := mt.Mirror.Repository
	logger := slog.Default().With(
		slog.Int64("mirror_id", mt.Mirror.ID),
		slog.Int64("mirror_task_id", mt.ID),
		slog.Int64("repository_id", repo.ID),
		slog.String("repo_path", fmt.Sprintf("%ss/%s", repo.RepositoryType, repo.Path)),
	)
	ctx = context.WithValue(ctx, lfsLoggerKey{}, logger)
	ctx = context.WithValue(ctx, rk, fmt.Sprintf("%ss/%s", repo.RepositoryType, repo.Path))
	ctx = context.WithValue(ctx, suk, mt.Mirror.SourceUrl)
	ctx = context.WithValue(ctx, sunk, mt.Mirror.Username)
	ctx = context.WithValue(ctx, satk, mt.Mirror.AccessToken)
	return context.WithValue(ctx, dbk, repo.DefaultBranch)
}

// loggerFromLFSContext returns the task-aware logger attached by SyncLFS.
func loggerFromLFSContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(lfsLoggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// lfsPointersToMetaObjects converts Git LFS pointers to de-duplicated database rows.
func lfsPointersToMetaObjects(repoID int64, lfsPointers []*types.LFSPointer) ([]database.LfsMetaObject, int64) {
	seen := make(map[string]struct{}, len(lfsPointers))
	lfsMetaObjects := make([]database.LfsMetaObject, 0, len(lfsPointers))
	var totalSize int64

	for _, lfsPointer := range lfsPointers {
		if lfsPointer == nil {
			continue
		}
		oid := lfsPointer.FileOid
		size := lfsPointer.FileSize
		if oid == "" {
			oid = lfsPointer.Oid
			size = lfsPointer.Size
		}
		if oid == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", repoID, oid)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		lfsMetaObjects = append(lfsMetaObjects, database.LfsMetaObject{
			Size:         size,
			Oid:          oid,
			RepositoryID: repoID,
			Existing:     false,
		})
		totalSize += size
	}

	return lfsMetaObjects, totalSize
}

// SyncLfs transfers missing LFS objects and publishes the commit produced by the Repo stage.
func (w *LfsSyncWorker) SyncLfs(ctx context.Context, mt *database.MirrorTask) error {
	var pointers []*types.Pointer

	if mt.Mirror == nil || mt.Mirror.Repository == nil {
		return fmt.Errorf("invalid mirror task")
	}

	mirror := mt.Mirror
	repo := mt.Mirror.Repository

	loggerFromLFSContext(ctx).InfoContext(ctx, "start to sync lfs")

	// Send message
	err := w.sendMessage(ctx, mt.Mirror, types.MirrorLfsSyncStart)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx, "failed to send notice message",
			slog.Any("error", err),
		)
	}

	pointers, err = w.getSyncPointers(ctx, mt)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx, "fail to get sync pointers",
			slog.Any("error", err),
		)
		return err
	}

	if len(pointers) > 0 {
		pointerGroups := SplitPointersBySizeAndCount(pointers)
		err = w.downloadAndUploadLFSFiles(ctx, mt, mirror, pointerGroups, repo)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx, "fail to download and upload lfs files",
				slog.Any("error", err),
			)

			return fmt.Errorf("fail to download and upload lfs files: %w", err)
		}
	}

	// Get repo last commit
	namespace, name, err := common.GetNamespaceAndNameFromPath(repo.Path)
	if err != nil {
		return fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
	}
	relativePath := repo.GitalyPath()

	commit, err := w.getRepoLastCommit(
		ctx, namespace, name, repo.DefaultBranch, repo.RepositoryType, relativePath,
	)
	if err != nil {
		return fmt.Errorf("failed to get repo last commit: %w", err)
	}

	if commit.ID != mt.AfterLastCommitID {
		// Point HEAD to new commit, so the uesrs can clone the changes
		loggerFromLFSContext(ctx).InfoContext(ctx,
			"Point HEAD to new commit",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
			slog.Any("commit_id", mt.AfterLastCommitID),
		)

		err = w.git.UpdateRef(ctx, gitserver.UpdateRefReq{
			Namespace:    namespace,
			Name:         name,
			Ref:          fmt.Sprintf("refs/heads/%s", repo.DefaultBranch),
			RepoType:     mirror.Repository.RepositoryType,
			NewObjectId:  mt.AfterLastCommitID,
			RelativePath: relativePath,
		})
		if err != nil {
			return fmt.Errorf("failed to point HEAD to new commit: %w", err)
		}
		loggerFromLFSContext(ctx).InfoContext(ctx,
			"Point HEAD to new commit successfully",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
			slog.Any("commit_id", mt.AfterLastCommitID),
		)
	}

	lastCommit, err := w.getRepoLastCommit(
		ctx, namespace, name, repo.DefaultBranch, repo.RepositoryType, relativePath,
	)
	if err != nil {
		return fmt.Errorf("failed to get repo last commit: %w", err)
	}

	// Trigger git callback
	err = w.triggerGitCallback(ctx, namespace, name, repo.DefaultBranch, lastCommit, repo, relativePath)
	if err != nil {
		return fmt.Errorf("failed to trigger git callback: %w", err)
	}

	return nil
}

func (w *LfsSyncWorker) getSyncPointers(
	ctx context.Context,
	mt *database.MirrorTask,
) ([]*types.Pointer, error) {
	var pointers []*types.Pointer
	// Query all lfsMetaObjects to generate the &types.Pointer slice
	lfsMetaObjects, err := w.lfsMetaObjectStore.FindByRepoID(ctx, mt.Mirror.Repository.ID)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"fail to get lfs meta objects",
			slog.Any("error", err),
		)
		return pointers, fmt.Errorf("fail to get lfs meta objects: %w", err)
	}
	repo := mt.Mirror.Repository

	loggerFromLFSContext(ctx).InfoContext(ctx,
		"fetched lfs meta objects",
		slog.Int("lfsCount", len(lfsMetaObjects)),
	)

	if len(lfsMetaObjects) == 0 {
		loggerFromLFSContext(ctx).InfoContext(ctx, "no lfs files to sync, finish sync lfs")
		return pointers, nil
	}

	var (
		existingOIDs []string
		missingOIDs  []string
	)
	for _, lfsMetaObject := range lfsMetaObjects {
		objectKey := common.BuildLfsPath(repo.ID, lfsMetaObject.Oid, repo.Migrated)
		exists, err := w.CheckIfLFSFileExists(ctx, objectKey, lfsMetaObject.Size)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to check if lfs file exists",
				slog.Any("error", err),
				slog.Any("objectKey", objectKey),
				slog.Any("repoType", repo.RepositoryType),
			)
			return pointers, fmt.Errorf("failed to check if lfs file exists: %w", err)
		}
		if exists {
			existingOIDs = append(existingOIDs, lfsMetaObject.Oid)
		} else {
			missingOIDs = append(missingOIDs, lfsMetaObject.Oid)
			pointers = append(pointers, &types.Pointer{
				Oid:  lfsMetaObject.Oid,
				Size: lfsMetaObject.Size,
			})
		}
	}

	loggerFromLFSContext(ctx).InfoContext(ctx,
		"checked lfs meta objects",
		slog.Int("lfsCount", len(lfsMetaObjects)),
		slog.Int("existingCount", len(existingOIDs)),
		slog.Int("missingCount", len(missingOIDs)),
	)

	if err = w.lfsMetaObjectStore.BulkUpdateExistingByOIDs(ctx, repo.ID, existingOIDs, missingOIDs); err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to update lfs meta objects",
			slog.Any("error", err),
		)
		return pointers, fmt.Errorf("failed to update lfs meta objects: %w", err)
	}

	if len(pointers) == 0 {
		loggerFromLFSContext(ctx).InfoContext(ctx, "no lfs files to sync, finish sync lfs")
	}

	return pointers, nil
}

func (w *LfsSyncWorker) sendMessage(ctx context.Context, mirror *database.Mirror, status types.MirrorTaskStatus) error {
	statusToSend := reposyncer.MirrorStatusToMessageTypeMapping[status]
	if statusToSend == "" {
		return nil
	}

	syncInfo := types.SyncInfo{
		RemoteURL: mirror.SourceUrl,
		LocalURL: fmt.Sprintf(
			"%s/%ss/%s",
			w.config.Frontend.URL,
			mirror.Repository.RepositoryType,
			mirror.Repository.Path,
		),
		RepoType: mirror.Repository.RepositoryType,
		Path:     mirror.Repository.Path,
		Status:   statusToSend,
		Size:     mirror.Repository.LFSObjectsSize,
	}
	byteInfo, _ := json.Marshal(syncInfo)
	message := types.MessageRequest{
		Scenario:   types.MessageScenarioRepoSync,
		Parameters: string(byteInfo),
		Priority:   types.MessagePriorityNormal,
	}
	resp, err := w.msgSender.Send(ctx, message)
	if err != nil {
		return err
	}
	loggerFromLFSContext(ctx).InfoContext(ctx,
		"send message",
		slog.Any("response", resp),
		slog.Any("syncInfo", syncInfo),
	)
	return nil
}

func (w *LfsSyncWorker) downloadAndUploadLFSFiles(
	ctx context.Context,
	mt *database.MirrorTask,
	mirror *database.Mirror,
	pointerGroups [][]*types.Pointer,
	repo *database.Repository,
) error {
	var finalErr error
	var pendingBytes int64
	for _, pointers := range pointerGroups {
		for _, pointer := range pointers {
			if pointer != nil && pointer.Size > 0 {
				pendingBytes += pointer.Size
			}
		}
	}
	totalBytes := repo.LFSObjectsSize
	if totalBytes < pendingBytes {
		totalBytes = pendingBytes
	}
	progress := newMirrorTaskByteProgress(mt, w.mirrorTaskStore, totalBytes, totalBytes-pendingBytes)
	if err := progress.persistInitial(ctx); err != nil {
		return err
	}

	for _, pointers := range pointerGroups {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pointers, err := w.GetLFSDownloadURLs(
			ctx, mirror.SourceUrl, repo.DefaultBranch, mirror.Username, mirror.AccessToken, pointers,
		)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to get lfs download urls",
				slog.Any("error", err),
				slog.Any("sourceURL", mirror.SourceUrl),
				slog.Any("repoType", repo.RepositoryType),
			)
			return fmt.Errorf("failed to get lfs download urls: %w", err)
		}

		for _, pointer := range pointers {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			err := w.downloadAndUploadLFSFile(ctx, repo, pointer, progress)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				finalErr = err
				loggerFromLFSContext(ctx).ErrorContext(ctx, "failed to download and upload lfs file",
					slog.Any("error", err),
					slog.Any("sourceURL", mirror.SourceUrl),
					slog.Any("repoType", repo.RepositoryType),
					slog.Any("pointer", pointer),
				)
				continue
			}
			if err := progress.completeObject(ctx, pointer.Oid, pointer.Size); err != nil {
				return err
			}
		}
	}
	if finalErr != nil {
		return finalErr
	}

	return nil
}

func (w *LfsSyncWorker) downloadAndUploadLFSFile(
	ctx context.Context,
	repo *database.Repository,
	pointer *types.Pointer,
	progress *mirrorTaskByteProgress,
) error {
	var uploadID string
	objectKey := common.BuildLfsPath(repo.ID, pointer.Oid, repo.Migrated)
	exists, err := w.CheckIfLFSFileExists(ctx, objectKey, pointer.Size)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to check if lfs file exists",
			slog.Any("error", err),
			slog.Any("objectKey", objectKey),
			slog.Any("repoType", repo.RepositoryType),
		)
		return fmt.Errorf("failed to check if lfs file exists: %w", err)
	}
	lmo := database.LfsMetaObject{
		Size:         pointer.Size,
		Oid:          pointer.Oid,
		RepositoryID: repo.ID,
		Existing:     exists,
	}

	_, err = w.lfsMetaObjectStore.UpdateOrCreate(ctx, lmo)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to update lfs meta object",
			slog.Any("error", err),
			slog.Any("lfsMetaObject", lmo),
			slog.Any("repoType", repo.RepositoryType),
		)
		return fmt.Errorf("failed to update lfs meta object: %w", err)
	}

	if exists {
		return nil
	}

	if pointer.DownloadURL == "" {
		loggerFromLFSContext(ctx).InfoContext(ctx, "pointer download url is empty",
			slog.Any("repoType", repo.RepositoryType),
		)
		return nil
	}

	if w.config.Mirror.PartSize <= 0 {
		return fmt.Errorf("LFS multipart part size must be positive: %d", w.config.Mirror.PartSize)
	}
	partSize := int64(w.config.Mirror.PartSize) * 1024 * 1024
	if pointer.Size/partSize > int64(maxPartNum) {
		partSize = pointer.Size / int64(maxPartNum)
	}
	repoPath := ctx.Value(rk).(string)

	if pointer.Size <= partSize {
		return w.downloadAndUploadSmallFile(ctx, repo, pointer, objectKey)
	}

	uploadID, err = w.syncCache.GetUploadID(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			loggerFromLFSContext(ctx).InfoContext(ctx, "upload lfs cache miss",
				slog.Any("repoType", repo.RepositoryType),
			)
		} else {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to get upload id from cache",
				slog.Any("error", err),
				slog.Any("repoType", repo.RepositoryType),
			)
		}
		uploadID = ""
	}

	if uploadID == "" {
		loggerFromLFSContext(ctx).InfoContext(ctx, "no upload id found in cache, creating new one")

		uploadID, err = w.ossCore.NewMultipartUpload(ctx, w.config.S3.Bucket, objectKey, minio.PutObjectOptions{
			PartSize: uint64(partSize),
		})
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to create new multipart upload",
				slog.Any("error", err),
			)
			return fmt.Errorf("failed to create new multipart upload: %w", err)
		}

		err = w.syncCache.CacheUploadID(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), uploadID)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to cache upload id",
				slog.Any("error", err),
			)
		}
	}

	err = w.multipartUploadWithRetry(
		ctx,
		partSize,
		uploadID,
		objectKey,
		w.config.Mirror.LfsConcurrency,
		repo.ID,
		pointer,
		progress,
	)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to upload object",
			slog.Any("uploadID", uploadID),
			slog.Any("objectKey", objectKey),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to upload object: %w", err)
	}

	info, err := w.ossClient.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to stat object %s: %w", objectKey, err)
	}

	if info.Size != pointer.Size {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"object size mismatch",
			slog.Any("objectKey", objectKey),
			slog.Any("expectedSize", pointer.Size),
			slog.Any("actualSize", info.Size),
		)
		err := w.syncCache.DeleteUploadID(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to delete upload id",
				slog.Any("error", err),
				slog.Any("uploadID", uploadID),
				slog.Any("oid", pointer.Oid),
			)
		}

		// delete the object if upload failed
		err = w.syncCache.DeleteLfsPartCache(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to delete lfs part cache",
				slog.Any("error", err),
				slog.Any("oid", pointer.Oid),
			)
		}

		// Reset lfs upload progress
		err = w.syncCache.CacheLfsSyncFileProgress(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), 0)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to reset lfs upload progress",
				slog.Any("error", err),
				slog.Any("oid", pointer.Oid),
			)
		}

		// delete the object if upload failed
		err = w.ossClient.RemoveObject(ctx, w.config.S3.Bucket, objectKey, minio.RemoveObjectOptions{})
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to remove object",
				slog.Any("error", err),
				slog.Any("objectKey", objectKey),
			)
		}

		return fmt.Errorf(
			"object size mismatch,repoPath: %s, oid: %s actually size  %d, expect size %d",
			repoPath, pointer.Oid, info.Size, pointer.Size)
	}

	lmo.Existing = true
	_, err = w.lfsMetaObjectStore.UpdateOrCreate(ctx, lmo)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to update lfs meta object existing",
			slog.Any("error", err),
			slog.Any("lfsMetaObject", lmo),
			slog.Any("repoType", repo.RepositoryType),
		)
		return fmt.Errorf("failed to update lfs meta object existing: %w", err)
	}

	// delete all cache if upload success
	err = w.syncCache.DeleteLfsSyncFileCache(ctx, repo.ID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to delete all cache",
			slog.Any("error", err),
			slog.Any("oid", pointer.Oid),
		)
	}

	return nil
}

// acquireUploadSlot waits for multipart upload capacity or context cancellation.
func acquireUploadSlot(ctx context.Context, slots <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-slots:
		return nil
	}
}

func (w *LfsSyncWorker) multipartUploadWithRetry(
	ctx context.Context,
	partSize int64,
	uploadID, objectKey string,
	concurrency int,
	repoID int64,
	pointer *types.Pointer,
	taskProgress *mirrorTaskByteProgress,
) error {
	if concurrency <= 0 {
		return fmt.Errorf("LFS multipart concurrency must be positive: %d", concurrency)
	}
	if partSize <= 0 {
		return fmt.Errorf("LFS multipart part size must be positive: %d", partSize)
	}

	var parts []minio.CompletePart
	eg, egCtx := errgroup.WithContext(ctx)
	// Use a channel to limit the number of concurrent uploads
	concurrencyChan := make(chan struct{}, concurrency)

	totalSize := pointer.Size

	downloadURL := pointer.DownloadURL
	if downloadURL == "" {
		return fmt.Errorf("empty download url for pointer: %s", pointer.Oid)
	}

	for i := 0; i < concurrency; i++ {
		concurrencyChan <- struct{}{}
	}
	partNumber0 := 1
	totalParts := int(math.Ceil(float64(totalSize) / float64(partSize)))

	uploadedParts, err := w.ossCore.ListObjectParts(ctx, w.config.S3.Bucket, objectKey, uploadID, 0, 0)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to list object parts",
			slog.Any("objectKey", objectKey),
			slog.Any("uploadID", uploadID),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to list object parts: %w", err)
	}
	// If the length of uploadedParts is more than totalParts, it means that the upload went wrong
	// and we need to abort the upload
	if len(uploadedParts.ObjectParts) > totalParts {
		err := w.ossCore.AbortMultipartUpload(ctx, w.config.S3.Bucket, objectKey, uploadID)
		if err != nil {
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to abort multipart upload",
				slog.Any("objectKey", objectKey),
				slog.Any("uploadID", uploadID),
				slog.Any("oid", pointer.Oid),
				slog.Any("err", err),
			)
			return fmt.Errorf("failed to abort multipart upload: %w", err)
		}
		return fmt.Errorf("uploaded part count exceeds expected total: %d/%d", len(uploadedParts.ObjectParts), totalParts)
	}

	existingPartNumbers := make(map[int]struct{}, len(uploadedParts.ObjectParts))
	var existingPartBytes int64
	for _, part := range uploadedParts.ObjectParts {
		existingPartNumbers[part.PartNumber] = struct{}{}
		existingPartBytes += part.Size
	}
	if err := taskProgress.addObjectBytes(ctx, pointer.Oid, pointer.Size, existingPartBytes); err != nil {
		return err
	}

	// Refresh cache progress
	progress := float64(len(uploadedParts.ObjectParts)) / float64(totalParts) * 100
	err = w.syncCache.CacheLfsSyncFileProgress(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), int(progress))
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to cache progress",
			slog.Any("objectKey", objectKey),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
	}

	var scheduleErr error
scheduleParts:
	for offset0 := int64(0); offset0 < totalSize; offset0 += partSize {
		select {
		case <-egCtx.Done():
			scheduleErr = context.Cause(egCtx)
			break scheduleParts
		default:
			offset := offset0
			partNumber := partNumber0
			// Acquire one upload slot while still allowing urgent preemption.
			if err := acquireUploadSlot(egCtx, concurrencyChan); err != nil {
				scheduleErr = err
				break scheduleParts
			}
			eg.Go(func() error {
				ctx := egCtx
				defer func() {
					concurrencyChan <- struct{}{}
				}()

				end := offset + partSize - 1
				if end > totalSize {
					end = totalSize - 1
				}

				synced, _ := w.syncCache.IsLfsPartSynced(
					ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), partNumber,
				)
				if synced {
					return nil
				}

				loggerFromLFSContext(ctx).InfoContext(ctx,
					"uploading part",
					slog.Any("objectKey", objectKey),
					slog.Any("partNumber", partNumber),
					slog.Any("offset", offset),
					slog.Any("end", end),
					slog.Any("totalSize", totalSize),
				)

				part, err := w.downloadAndUploadPartWithRetry(
					ctx, uploadID, objectKey, partNumber, offset, end, pointer)
				if err != nil {
					loggerFromLFSContext(ctx).ErrorContext(ctx,
						"failed to upload part",
						slog.Any("objectKey", objectKey),
						slog.Any("partNumber", partNumber),
						slog.Any("offset", offset),
						slog.Any("end", end),
						slog.Any("totalSize", totalSize),
						slog.Any("err", err),
					)
					return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
				}
				if _, alreadyUploaded := existingPartNumbers[partNumber]; !alreadyUploaded {
					if err := taskProgress.addObjectBytes(ctx, pointer.Oid, pointer.Size, end-offset+1); err != nil {
						return err
					}
				}

				w.mu.Lock()
				parts = append(parts, minio.CompletePart{
					ETag:       part.ETag,
					PartNumber: part.PartNumber,
				})
				w.mu.Unlock()

				loggerFromLFSContext(ctx).InfoContext(ctx,
					"caching part",
					slog.Any("objectKey", objectKey),
					slog.Any("partNumber", partNumber),
				)

				// Cache the part uploaded
				err = w.syncCache.CacheLfsSyncAddPart(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), partNumber)
				if err != nil {
					loggerFromLFSContext(ctx).ErrorContext(ctx,
						"failed to add part to cache",
						slog.Any("objectKey", objectKey),
						slog.Any("partNumber", partNumber),
						slog.Any("oid", pointer.Oid),
						slog.Any("err", err),
					)
				}

				// Count progress
				uploadedPartCount, err := w.syncCache.LfsPartSyncedCount(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
				if err != nil {
					loggerFromLFSContext(ctx).ErrorContext(ctx,
						"failed to count uploaded parts",
						slog.Any("objectKey", objectKey),
						slog.Any("oid", pointer.Oid),
						slog.Any("err", err),
					)
				}
				progress := float64(uploadedPartCount) / float64(totalParts) * 100
				err = w.syncCache.CacheLfsSyncFileProgress(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), int(progress))
				if err != nil {
					loggerFromLFSContext(ctx).ErrorContext(ctx,
						"failed to cache progress",
						slog.Any("objectKey", objectKey),
						slog.Any("oid", pointer.Oid),
						slog.Any("err", err),
					)
				}

				return nil
			})
			partNumber0++
		}
	}
	err = eg.Wait()
	if err == nil {
		err = scheduleErr
	}
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to download and upload lfs file",
			slog.Any("objectKey", objectKey),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to download and upload lfs file: %w", err)
	}

	result, err := w.ossCore.ListObjectParts(ctx, w.config.S3.Bucket, objectKey, uploadID, 0, 0)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to list object parts",
			slog.Any("objectKey", objectKey),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to list object parts: %w", err)
	}

	if len(result.ObjectParts) != totalParts {
		// If the number of parts in OSS is more than the parts we caculated, should delete the uploadID and retry the upload
		if len(result.ObjectParts) > totalParts {
			err := w.syncCache.DeleteUploadID(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize))
			if err != nil {
				loggerFromLFSContext(ctx).ErrorContext(ctx,
					"failed to delete upload id",
					slog.Any("error", err),
					slog.Any("uploadID", uploadID),
					slog.Any("oid", pointer.Oid),
				)
			}
			return fmt.Errorf("upload more parts than expected, expected: %d, got: %d",
				totalParts, len(result.ObjectParts))
		}
		partNumberMapping := make(map[int]struct{})
		for _, part := range result.ObjectParts {
			partNumberMapping[part.PartNumber] = struct{}{}
		}
		for partNumber := 1; partNumber <= int(totalParts); partNumber++ {
			_, ok := partNumberMapping[partNumber]
			if !ok {
				loggerFromLFSContext(ctx).ErrorContext(ctx, "part number missing", slog.Any("partNumber", partNumber), slog.Any("oid", pointer.Oid))
				err := w.syncCache.DeleteSpecificLfsPartCache(ctx, repoID, pointer.Oid, strconv.Itoa(w.config.Mirror.PartSize), partNumber)
				if err != nil {
					loggerFromLFSContext(ctx).ErrorContext(ctx,
						"failed to delete specific lfs part cache",
						slog.Any("error", err),
						slog.Any("partNumber", partNumber),
						slog.Any("oid", pointer.Oid),
						slog.Any("error", err),
					)
				}
			}
		}
		return fmt.Errorf("not all parts uploaded, %d/%d", len(result.ObjectParts), totalParts)
	}

	// // Sort the parts by part number
	// sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

	var completeCompleteParts []minio.CompletePart
	for _, part := range result.ObjectParts {
		completeCompleteParts = append(completeCompleteParts, minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}

	sort.Slice(completeCompleteParts, func(i, j int) bool { return completeCompleteParts[i].PartNumber < completeCompleteParts[j].PartNumber })
	_, err = w.ossCore.CompleteMultipartUpload(
		ctx, w.config.S3.Bucket, objectKey, uploadID, completeCompleteParts, minio.PutObjectOptions{
			DisableContentSha256: true,
		},
	)
	if err != nil {
		loggerFromLFSContext(ctx).ErrorContext(ctx,
			"failed to complete multipart upload",
			slog.Any("error", err),
			slog.Any("uploadID", uploadID),
			slog.Any("objectKey", objectKey),
		)
		return fmt.Errorf("failed to complete multipart upload, %w", err)
	}

	loggerFromLFSContext(ctx).InfoContext(ctx,
		"complete multipart upload",
		slog.Any("error", err),
		slog.Any("uploadID", uploadID),
		slog.Any("objectKey", objectKey),
	)
	return nil
}

func (w *LfsSyncWorker) downloadAndUploadSmallFile(
	ctx context.Context,
	repo *database.Repository,
	pointer *types.Pointer,
	objectKey string,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	loggerFromLFSContext(ctx).InfoContext(ctx,
		"downloading small file directly",
		slog.Any("oid", pointer.Oid),
		slog.Any("size", pointer.Size),
	)

	resp, err := w.downloadRange(ctx, pointer.DownloadURL, pointer.DownloadHeaders, 0, pointer.Size-1)
	if err != nil {
		return fmt.Errorf("failed to download small file: %w", err)
	}
	defer resp.Body.Close()

	_, err = w.ossClient.PutObject(ctx, w.config.S3.Bucket, objectKey, resp.Body, pointer.Size, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	info, err := w.ossClient.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to stat object %s: %w", objectKey, err)
	}

	if info.Size != pointer.Size {
		err := w.ossClient.RemoveObject(ctx, w.config.S3.Bucket, objectKey, minio.RemoveObjectOptions{})
		if err != nil {
			loggerFromLFSContext(ctx).WarnContext(ctx, "failed to remove mismatched object", slog.Any("error", err))
		}
		return fmt.Errorf(
			"object size mismatch, oid: %s actually size %d, expect size %d",
			pointer.Oid, info.Size, pointer.Size)
	}

	lmo := database.LfsMetaObject{
		Size:         pointer.Size,
		Oid:          pointer.Oid,
		RepositoryID: repo.ID,
		Existing:     true,
	}
	_, err = w.lfsMetaObjectStore.UpdateOrCreate(ctx, lmo)
	if err != nil {
		return fmt.Errorf("failed to update lfs meta object existing: %w", err)
	}

	return nil
}

func (w *LfsSyncWorker) downloadAndUploadPartWithRetry(
	ctx context.Context,
	uploadID, objectKey string,
	partNumber int,
	start int64,
	end int64,
	pointer *types.Pointer,
) (minio.ObjectPart, error) {
	var (
		part    minio.ObjectPart
		lastErr error
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		downloadURL := pointer.DownloadURL
		if downloadURL == "" {
			return part, fmt.Errorf("downloadURL is empty")
		}
		loggerFromLFSContext(ctx).InfoContext(ctx,
			"downloading range",
			slog.Any("objectKey", objectKey),
			slog.Any("downloadURL", downloadURL),
			slog.Any("partNumber", partNumber),
			slog.Any("offset", start),
			slog.Any("end", end),
		)
		resp, err := w.downloadRange(ctx, downloadURL, pointer.DownloadHeaders, start, end)
		if err != nil {
			if ctx.Err() != nil {
				return part, context.Cause(ctx)
			}
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to download range",
				slog.Any("downloadURL", downloadURL),
				slog.Any("partNumber", partNumber),
				slog.Any("start", start),
				slog.Any("end", end),
				slog.Any("attempt", attempt),
				slog.Any("error", err),
			)
			var statusErr *unexpectedHTTPStatusError
			if errors.As(err, &statusErr) && statusErr.statusCode == http.StatusForbidden {
				sourceURL := ctx.Value(suk).(string)
				defaultBranch := ctx.Value(dbk).(string)
				username, _ := ctx.Value(sunk).(string)
				accessToken, _ := ctx.Value(satk).(string)
				pointers, err := w.GetLFSDownloadURLs(
					ctx, sourceURL, defaultBranch, username, accessToken, []*types.Pointer{pointer},
				)
				if err != nil {
					return part, fmt.Errorf("failed to get download URLs: %w", err)
				}
				pointer = pointers[0]
				continue
			}
			return part, fmt.Errorf("failed to download range: %w", err)
		}

		defer resp.Body.Close()

		loggerFromLFSContext(ctx).InfoContext(ctx,
			"uploading range",
			slog.Any("objectKey", objectKey),
			slog.Any("partNumber", partNumber),
			slog.Any("offset", start),
			slog.Any("end", end),
		)
		part, err = w.ossCore.PutObjectPart(
			ctx,
			w.config.S3.Bucket,
			objectKey,
			uploadID,
			partNumber,
			resp.Body,
			resp.ContentLength,
			minio.PutObjectPartOptions{
				DisableContentSha256: true,
			},
		)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return part, context.Cause(ctx)
			}
			loggerFromLFSContext(ctx).ErrorContext(ctx,
				"failed to upload range",
				slog.Any("objectKey", objectKey),
				slog.Any("partNumber", partNumber),
				slog.Any("offset", start),
				slog.Any("end", end),
				slog.Any("attempt", attempt),
				slog.Any("error", err),
			)

			if attempt == maxRetries {
				break
			}
			timer := time.NewTimer(3 * time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return part, context.Cause(ctx)
			case <-timer.C:
			}
			continue
		}
		return part, nil
	}

	return part, fmt.Errorf(
		"failed to download and upload part %d after %d attempts: %w",
		partNumber,
		maxRetries,
		lastErr,
	)
}

// downloadRange downloads one byte range with the headers supplied by the LFS download action.
func (w *LfsSyncWorker) downloadRange(
	ctx context.Context,
	downloadURL string,
	downloadHeaders http.Header,
	start, end int64,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LFS download URL: %w", err)
	}

	req.Header.Set("Host", parsedURL.Host)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json; charset=utf-8")
	req.Header.Set("User-Agent", "git-lfs/3.5.1")
	for name, values := range downloadHeaders {
		req.Header.Del(name)
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Set("Range", rangeHeader)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &unexpectedHTTPStatusError{statusCode: resp.StatusCode}
	}

	return resp, nil
}

func (w *LfsSyncWorker) CheckIfLFSFileExists(
	ctx context.Context,
	objectKey string,
	size int64,
) (bool, error) {
	objInfo, err := w.ossClient.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if isLFSObjectNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check if the file size matches the expected size
	if objInfo.Size != size {
		// Delete the mismatched object
		if err := w.ossClient.RemoveObject(ctx, w.config.S3.Bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
			return false, fmt.Errorf("failed to remove mismatched object: %w", err)
		}
		return false, nil
	}

	return true, nil
}

// isLFSObjectNotFound detects missing LFS objects from structured S3 errors first, with message matching as a compatibility fallback.
func isLFSObjectNotFound(err error) bool {
	if err == nil {
		return false
	}

	minioErr := minio.ToErrorResponse(err)
	if minioErr.Code == "NoSuchKey" {
		return true
	}
	if minioErr.Code == "" && minioErr.StatusCode == http.StatusNotFound {
		return true
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "nosuchkey") ||
		strings.Contains(errMsg, "key does not exist") ||
		strings.Contains(errMsg, "object does not exist") ||
		strings.Contains(errMsg, "object not found")
}

// GetLFSDownloadURLs requests download actions from the source repository LFS Batch API.
func (w *LfsSyncWorker) GetLFSDownloadURLs(
	ctx context.Context, repoCloneURL, branch, username, accessToken string,
	pointers []*types.Pointer,
) ([]*types.Pointer, error) {
	var (
		resPointers []*types.Pointer
		lfsAPIURL   string
	)
	requestPayload := types.LFSBatchRequest{
		Operation: "download",
	}

	for _, pointer := range pointers {
		requestPayload.Objects = append(requestPayload.Objects, types.LFSBatchObject{
			Oid:  pointer.Oid,
			Size: pointer.Size,
		})
	}

	requestPayload.HashAlog = "sha256"
	requestPayload.Transfers = []string{"lfs-standalone-file", "basic", "bash"}
	requestPayload.Ref = types.LFSBatchObjectRef{
		Name: fmt.Sprintf("refs/heads/%s", branch),
	}

	if strings.HasSuffix(repoCloneURL, ".git") {
		lfsAPIURL = repoCloneURL + "/info/lfs/objects/batch"
	} else {
		lfsAPIURL = repoCloneURL + ".git/info/lfs/objects/batch"
	}

	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, lfsAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	parsedURL, err := url.Parse(lfsAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LFS API URL: %w", err)
	}

	req.Header.Set("Host", parsedURL.Host)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json; charset=utf-8")
	req.Header.Set("User-Agent", "git-lfs/3.5.1")
	if username != "" && accessToken != "" {
		req.SetBasicAuth(username, accessToken)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get lfs download url, unexpected status code: %d", resp.StatusCode)
	}

	var batchResp types.BatchResponse
	err = json.NewDecoder(resp.Body).Decode(&batchResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	if len(batchResp.Objects) == 0 {
		return nil, fmt.Errorf("no objects returned in batch response")
	}

	for _, obj := range batchResp.Objects {
		var (
			downloadURL     string
			downloadHeaders http.Header
		)
		// Some objects may be unreachable or been removed
		downloadAction := obj.Actions["download"]
		if downloadAction == nil {
			loggerFromLFSContext(ctx).WarnContext(ctx, "download URL not found for object", slog.Any("obj", obj.Oid))
			downloadURL = ""
		} else {
			downloadURL = downloadAction.Href
			downloadHeaders = make(http.Header, len(downloadAction.Header))
			for name, value := range downloadAction.Header {
				headerValue, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("invalid LFS download header %q for object %s", name, obj.Oid)
				}
				downloadHeaders.Set(name, headerValue)
			}
		}
		resPointers = append(resPointers, &types.Pointer{
			Oid:             obj.Oid,
			Size:            obj.Size,
			DownloadURL:     downloadURL,
			DownloadHeaders: downloadHeaders,
		})
	}

	return resPointers, nil
}

// getRepoLastCommit resolves a commit without requiring Gitaly to query repository metadata.
func (w *LfsSyncWorker) getRepoLastCommit(
	ctx context.Context,
	namespace, name, branch string,
	repoType types.RepositoryType,
	relativePath string,
) (*types.Commit, error) {
	commit, err := w.git.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace:    namespace,
		Name:         name,
		RepoType:     repoType,
		Ref:          branch,
		RelativePath: relativePath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get repo last commit: %w", err)
	}
	return commit, nil
}

// triggerGitCallback submits the synchronized Git diff to the repository callback workflow.
func (w *LfsSyncWorker) triggerGitCallback(
	ctx context.Context,
	namespace, name, branch string,
	commit *types.Commit,
	repo *database.Repository,
	relativePath string,
) error {
	callback, err := w.git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     namespace,
		Name:          name,
		RepoType:      repo.RepositoryType,
		Ref:           branch,
		LeftCommitId:  gitaly.SHA1EmptyTreeID,
		RightCommitId: commit.ID,
		Private:       repo.Private,
		RelativePath:  relativePath,
	})
	if err != nil {
		return fmt.Errorf("failed to get diff between two commits: %w", err)
	}
	callback.Ref = branch

	//start workflow to handle push request

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
		ID:        fmt.Sprintf("mirror-lfs-%s-%s-%s-%s", repo.RepositoryType, namespace, name, commit.ID),
	}

	_, err = w.workflowClient.ExecuteWorkflow(
		ctx, workflowOptions, workflow.HandlePushWorkflow, callback,
	)
	if err != nil {
		return fmt.Errorf("failed to handle git push callback: %w", err)
	}

	loggerFromLFSContext(ctx).InfoContext(ctx,
		"start handle push workflow",
		// slog.String("workflowID", we.GetID()),
		slog.Any("req", callback),
		slog.Any("repoType", repo.RepositoryType))

	return nil
}

func SplitPointersBySizeAndCount(pointers []*types.Pointer) [][]*types.Pointer {
	var groups [][]*types.Pointer
	var currentGroup []*types.Pointer
	var currentSize int64 = 0

	for _, p := range pointers {
		if currentSize+p.Size > maxGroupSize || len(currentGroup) >= maxGroupCount {
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []*types.Pointer{}
			currentSize = 0
		}

		currentGroup = append(currentGroup, p)
		currentSize += p.Size
	}

	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}
