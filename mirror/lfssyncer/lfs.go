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
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"go.temporal.io/sdk/client"
	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/mirror/filter"
)

type repoPathKey string

var (
	rk             repoPathKey = "repoPath"
	maxRetries     int         = 3
	MaxGroupSize   int64       = 10 * 1024 * 1024 * 1024 // 10GB
	MaxGroupCount  int         = 15
	maxPartNum     int         = 1000
	lfsConcurrency             = 5
	lfsPartSize                = 100
)

type LfsSyncWorker struct {
	id                 int
	wg                 *sync.WaitGroup
	mirrorStore        database.MirrorStore
	mirrorTaskStore    database.MirrorTaskStore
	lfsMetaObjectStore database.LfsMetaObjectStore
	repoStore          database.RepoStore
	ossClient          s3.Client
	ossCore            s3.Core
	config             *config.Config
	syncCache          cache.Cache
	mu                 sync.Mutex
	httpClient         *http.Client
	recomComponent     component.RecomComponent
	ctx                context.Context
	repoFilter         *filter.RepoFilter
	git                gitserver.GitServer
}

func NewLfsSyncWorker(config *config.Config, id int) (*LfsSyncWorker, error) {
	var err error
	w := &LfsSyncWorker{
		id: id,
		wg: &sync.WaitGroup{},
	}
	w.config = config
	w.repoFilter = filter.NewRepoFilter(config)
	w.mirrorStore = database.NewMirrorStore()
	w.mirrorTaskStore = database.NewMirrorTaskStore()
	w.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	w.repoStore = database.NewRepoStore()
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

	recomComponent, err := component.NewRecomComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create recom component")
	}
	w.recomComponent = recomComponent

	w.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	return w, nil
}

func (w *LfsSyncWorker) SetContext(ctx context.Context) {
	w.ctx = ctx
}

func (w *LfsSyncWorker) ID() int {
	return w.id
}

func (w *LfsSyncWorker) Run(mt *database.MirrorTask) {
	var action string
	mirror, err := w.mirrorStore.FindByID(w.ctx, mt.MirrorID)
	if err != nil {
		slog.Error(
			"fail to get mirror",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		mt.ErrorMessage = "mirror not found"
		mt.Status = types.MirrorLfsSyncFailed
		_, updateErr := w.mirrorTaskStore.Update(w.ctx, *mt)
		if updateErr != nil {
			slog.Error("fail to update mirror task",
				slog.Int("workerID", w.id),
				slog.Any("error", updateErr),
			)
		}
		return
	}

	_, err = w.repoStore.FindById(w.ctx, mirror.RepositoryID)
	if err != nil {
		slog.Error("fail to get repo",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		return
	}

	repoPath := fmt.Sprintf("%ss/%s", mirror.Repository.RepositoryType, mirror.Repository.Path)
	w.ctx = context.WithValue(w.ctx, rk, repoPath)

	err = w.SyncLfs(w.ctx, mt)
	if err != nil {
		slog.Error("fail to sync lfs",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		if errors.Is(err, context.Canceled) {
			action = database.MirrorCancel
		} else {
			action = database.MirrorFail
		}
		mt.ErrorMessage = err.Error()
	} else {
		action = database.MirrorSuccess
		mt.Progress = 100
	}

	mtFSM := database.NewMirrorTaskWithFSM(mt)
	// Can not use w.ctx cause it could be canceled
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	canContinue := mtFSM.SubmitEvent(ctx, action)
	if !canContinue {
		slog.Error("fail to submit event",
			slog.Int("workerID", w.id),
			slog.Any("status", mt.Status),
			slog.Any("action", action),
		)

		mt.ErrorMessage = fmt.Sprintf("fail to submit event, status: %s, action: %s", mt.Status, action)
		mt.Status = types.MirrorLfsSyncFailed
		_, updateErr := w.mirrorTaskStore.Update(w.ctx, *mt)
		if updateErr != nil {
			slog.Error("fail to update mirror task",
				slog.Int("workerID", w.id),
				slog.Any("error", updateErr),
			)
		}
		return
	}
	mt.Status = types.MirrorTaskStatus(mtFSM.Current())
	_, err = w.mirrorTaskStore.Update(ctx, *mt)
	if err != nil {
		slog.Error("fail to update mirror task",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		return
	}

	err = w.recomComponent.SetOpWeight(w.ctx, mirror.RepositoryID, int64(100*mt.Priority))
	if err != nil {
		slog.Error("fail to set op weight",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("repoID", mt.Mirror.RepositoryID),
			slog.Any("repoPath", repoPath),
		)
	}
}

func (w *LfsSyncWorker) SyncLfs(ctx context.Context, mt *database.MirrorTask) error {
	var pointers []*types.Pointer

	if mt.Mirror == nil || mt.Mirror.Repository == nil {
		return fmt.Errorf("invalid mirror task")
	}

	mirror := mt.Mirror
	repo := mt.Mirror.Repository

	repoPath := ctx.Value(rk).(string)

	slog.Info("start to sync lfs",
		slog.Int("workerID", w.id),
		slog.Any("mirrorTaskID", mt.ID),
		slog.Any("repoPath", repoPath),
	)

	pointers, err := w.getSyncPointers(ctx, mt)
	if err != nil {
		slog.Error("fail to get sync pointers",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		return err
	}

	if len(pointers) == 0 {
		return nil
	}

	pointerGroups := SplitPointersBySizeAndCount(pointers)
	err = w.downloadAndUploadLFSFiles(ctx, mt, mirror, pointerGroups, repo)
	if err != nil {
		slog.Error("fail to download and upload lfs files",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)

		return fmt.Errorf("fail to download and upload lfs files: %w", err)
	}

	// Get repo last commit
	namespace, name, err := common.GetNamespaceAndNameFromPath(repo.Path)
	if err != nil {
		return fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
	}

	commit, err := w.getRepoLastCommit(
		ctx, namespace, name, repo.DefaultBranch, repo.RepositoryType,
	)
	if err != nil {
		return fmt.Errorf("failed to get repo last commit: %w", err)
	}

	if commit.ID != mt.AfterLastCommitID {
		// Point HEAD to new commit, so the uesrs can clone the changes
		slog.Info(
			"Point HEAD to new commit",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
			slog.Any("commit_id", mt.AfterLastCommitID),
		)

		err = w.git.UpdateRef(ctx, gitserver.UpdateRefReq{
			Namespace:   namespace,
			Name:        name,
			Ref:         fmt.Sprintf("refs/heads/%s", repo.DefaultBranch),
			RepoType:    mirror.Repository.RepositoryType,
			NewObjectId: mt.AfterLastCommitID,
		})
		if err != nil {
			return fmt.Errorf("failed to point HEAD to new commit: %w", err)
		}
		slog.Info(
			"Point HEAD to new commit successfully",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
			slog.Any("commit_id", mt.AfterLastCommitID),
		)
	}

	lastCommit, err := w.getRepoLastCommit(
		ctx, namespace, name, repo.DefaultBranch, repo.RepositoryType,
	)
	if err != nil {
		return fmt.Errorf("failed to get repo last commit: %w", err)
	}

	// Trigger git callback
	err = w.triggerGitCallback(ctx, namespace, name, repo.DefaultBranch, lastCommit, repo)
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
	repoPath := ctx.Value(rk).(string)
	// Query all lfsMetaObjects to generate the &types.Pointer slice
	lfsMetaObjects, err := w.lfsMetaObjectStore.FindByRepoID(ctx, mt.Mirror.Repository.ID)
	if err != nil {
		slog.Error(
			"fail to get lfs meta objects",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
		)
		return pointers, fmt.Errorf("fail to get lfs meta objects: %w", err)
	}
	for _, lfsMetaObject := range lfsMetaObjects {
		if !lfsMetaObject.Existing {
			pointers = append(pointers, &types.Pointer{
				Oid:  lfsMetaObject.Oid,
				Size: lfsMetaObject.Size,
			})
		}
	}
	if len(pointers) == 0 {
		slog.Info("no lfs files to sync, finish sync lfs", slog.Int("workerId", w.id), slog.String("repoPath", repoPath))
	}

	return pointers, nil
}

func (w *LfsSyncWorker) downloadAndUploadLFSFiles(
	ctx context.Context,
	mt *database.MirrorTask,
	mirror *database.Mirror,
	pointerGroups [][]*types.Pointer,
	repo *database.Repository,
) error {
	totalPointerCount := 0
	syncedPointerCount := 0

	for _, pointers := range pointerGroups {
		totalPointerCount += len(pointers)
	}

	for _, pointers := range pointerGroups {
		pointers, err := w.GetLFSDownloadURLs(ctx, mirror.SourceUrl, repo.DefaultBranch, pointers)
		if err != nil {
			slog.Error(
				"failed to get lfs download urls",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("sourceURL", mirror.SourceUrl),
				slog.Any("repoPath", repo.Path),
				slog.Any("repoType", repo.RepositoryType),
			)
			return fmt.Errorf("failed to get lfs download urls: %w", err)
		}

		for _, pointer := range pointers {
			err := w.downloadAndUploadLFSFile(ctx, repo, pointer)
			if err != nil {
				return fmt.Errorf("failed to download and upload lfs file: %w", err)
			}

			syncedPointerCount++
			// Update the progress of the mirror task
			mt.Progress = int(math.Ceil(float64(syncedPointerCount) / float64(totalPointerCount) * 100))
			_, err = w.mirrorTaskStore.Update(ctx, *mt)
			if err != nil {
				return fmt.Errorf("failed to update mirror task progress: %w", err)
			}
		}
	}

	return nil
}

func (w *LfsSyncWorker) downloadAndUploadLFSFile(
	ctx context.Context,
	repo *database.Repository,
	pointer *types.Pointer,
) error {
	var uploadID string
	objectKey := common.BuildLfsPath(repo.ID, pointer.Oid, repo.Migrated)
	exists, err := w.CheckIfLFSFileExists(ctx, objectKey)
	if err != nil {
		slog.Error(
			"failed to check if lfs file exists",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("objectKey", objectKey),
			slog.Any("repoPath", repo.Path),
			slog.Any("repoType", repo.RepositoryType),
		)
	}
	lmo := database.LfsMetaObject{
		Size:         pointer.Size,
		Oid:          pointer.Oid,
		RepositoryID: repo.ID,
		Existing:     exists,
	}

	_, err = w.lfsMetaObjectStore.UpdateOrCreate(ctx, lmo)
	if err != nil {
		slog.Error(
			"failed to update lfs meta object",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("lfsMetaObject", lmo),
			slog.Any("repoPath", repo.Path),
			slog.Any("repoType", repo.RepositoryType),
		)
		return fmt.Errorf("failed to update lfs meta object: %w", err)
	}

	if exists {
		return nil
	}

	if pointer.DownloadURL == "" {
		return fmt.Errorf(
			"pointer download url is empty, repoPath: %s, repoType: %s",
			repo.Path,
			repo.RepositoryType,
		)
	}

	partSize := int64(lfsPartSize * 1024 * 1024)
	if pointer.Size/partSize > int64(maxPartNum) {
		partSize = pointer.Size / int64(maxPartNum)
	}
	repoPath := ctx.Value(rk).(string)

	uploadID, err = w.syncCache.GetUploadID(ctx, repoPath, pointer.Oid)
	if err != nil {
		slog.Error(
			"failed to get upload id from cache",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("repoPath", repoPath),
			slog.Any("repoType", repo.RepositoryType),
		)
	}

	if uploadID == "" {
		slog.Info(
			"no upload id found in cache, creating new one",
			slog.Int("workerID", w.id),
			slog.Any("repoPath", repoPath),
		)

		uploadID, err = w.ossCore.NewMultipartUpload(ctx, w.config.S3.Bucket, objectKey, minio.PutObjectOptions{
			PartSize: uint64(partSize),
		})
		if err != nil {
			slog.Error(
				"failed to create new multipart upload",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
			)
			return fmt.Errorf("failed to create new multipart upload: %w", err)
		}

		err = w.syncCache.CacheUploadID(ctx, repoPath, pointer.Oid, uploadID)
		if err != nil {
			slog.Error(
				"failed to cache upload id",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
			)
		}
	}

	err = w.multipartUploadWithRetry(
		ctx,
		partSize,
		uploadID,
		objectKey,
		lfsConcurrency,
		pointer,
	)
	if err != nil {
		slog.Error(
			"failed to upload object",
			slog.Int("workerID", w.id),
			slog.Any("uploadID", uploadID),
			slog.Any("objectKey", objectKey),
			slog.Any("error", err),
			slog.Any("repoPath", repoPath),
		)
		return fmt.Errorf("failed to upload object: %w", err)
	}

	info, err := w.ossClient.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to stat object %s: %w", objectKey, err)
	}

	if info.Size != pointer.Size {
		slog.Error(
			"object size mismatch",
			slog.Int("workerID", w.id),
			slog.Any("objectKey", objectKey),
			slog.Any("expectedSize", pointer.Size),
			slog.Any("actualSize", info.Size),
			slog.Any("repoPath", repoPath),
		)
		err := w.syncCache.DeleteUploadID(ctx, repoPath, pointer.Oid)
		if err != nil {
			slog.Error(
				"failed to delete upload id",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
				slog.Any("uploadID", uploadID),
				slog.Any("oid", pointer.Oid),
			)
		}

		// delete the object if upload failed
		err = w.syncCache.DeleteLfsPartCache(ctx, repoPath, pointer.Oid)
		if err != nil {
			slog.Error(
				"failed to delete lfs part cache",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
				slog.Any("oid", pointer.Oid),
			)
		}

		// Reset lfs upload progress
		err = w.syncCache.CacheLfsSyncFileProgress(ctx, repoPath, pointer.Oid, 0)
		if err != nil {
			slog.Error(
				"failed to reset lfs upload progress",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
				slog.Any("oid", pointer.Oid),
			)
		}

		// delete the object if upload failed
		err = w.ossClient.RemoveObject(ctx, w.config.S3.Bucket, objectKey, minio.RemoveObjectOptions{})
		if err != nil {
			slog.Error(
				"failed to remove object",
				slog.Int("workerID", w.id),
				slog.Any("error", err),
				slog.Any("repoPath", repoPath),
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
		slog.Error(
			"failed to update lfs meta object existing",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("lfsMetaObject", lmo),
			slog.Any("repoPath", repo.Path),
			slog.Any("repoType", repo.RepositoryType),
		)
		return fmt.Errorf("failed to update lfs meta object existing: %w", err)
	}

	// delete all cache if upload success
	err = w.syncCache.DeleteAllCache(ctx, repoPath, pointer.Oid)
	if err != nil {
		slog.Error(
			"failed to delete all cache",
			slog.Int("workerID", w.id),
			slog.Any("error", err),
			slog.Any("repoPath", repoPath),
			slog.Any("oid", pointer.Oid),
		)
	}

	return nil
}

func (w *LfsSyncWorker) multipartUploadWithRetry(
	ctx context.Context,
	partSize int64,
	uploadID, objectKey string,
	concurrency int,
	pointer *types.Pointer,
) error {
	var parts []minio.CompletePart
	repoPath := ctx.Value(rk).(string)
	eg := new(errgroup.Group)
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

	for offset0 := int64(0); offset0 < totalSize; offset0 += partSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			offset := offset0
			partNumber := partNumber0
			// Use errgroup.Group to get the first error of the goroutines
			<-concurrencyChan
			eg.Go(func() error {
				defer func() {
					concurrencyChan <- struct{}{}
				}()

				end := offset + partSize - 1
				if end > totalSize {
					end = totalSize - 1
				}

				synced, _ := w.syncCache.IsLfsPartSynced(
					ctx, repoPath, pointer.Oid, partNumber,
				)
				if synced {
					return nil
				}

				slog.Info(
					"uploading part",
					slog.Int("workerID", w.id),
					slog.Any("repoPath", repoPath),
					slog.Any("objectKey", objectKey),
					slog.Any("partNumber", partNumber),
					slog.Any("offset", offset),
					slog.Any("end", end),
					slog.Any("totalSize", totalSize),
				)

				part, err := w.downloadAndUploadPartWithRetry(
					ctx, downloadURL, uploadID, objectKey, partNumber, offset, end)
				if err != nil {
					slog.Error(
						"failed to upload part",
						slog.Int("workerID", w.id),
						slog.Any("repoPath", repoPath),
						slog.Any("objectKey", objectKey),
						slog.Any("partNumber", partNumber),
						slog.Any("offset", offset),
						slog.Any("end", end),
						slog.Any("totalSize", totalSize),
						slog.Any("err", err),
					)
					return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
				}

				w.mu.Lock()
				parts = append(parts, minio.CompletePart{
					ETag:       part.ETag,
					PartNumber: part.PartNumber,
				})
				w.mu.Unlock()

				slog.Info(
					"caching part",
					slog.Int("workerID", w.id),
					slog.Any("repoPath", repoPath),
					slog.Any("objectKey", objectKey),
					slog.Any("partNumber", partNumber),
				)

				// Cache the part uploaded
				err = w.syncCache.CacheLfsSyncAddPart(ctx, repoPath, pointer.Oid, partNumber)
				if err != nil {
					slog.Error(
						"failed to add part to cache",
						slog.Int("workerID", w.id),
						slog.Any("repoPath", repoPath),
						slog.Any("objectKey", objectKey),
						slog.Any("partNumber", partNumber),
						slog.Any("oid", pointer.Oid),
						slog.Any("err", err),
					)
				}

				// Count progress
				uploadedPartCount, err := w.syncCache.LfsPartSyncedCount(ctx, repoPath, pointer.Oid)
				if err != nil {
					slog.Error(
						"failed to count uploaded parts",
						slog.Int("workerID", w.id),
						slog.Any("repoPath", repoPath),
						slog.Any("objectKey", objectKey),
						slog.Any("oid", pointer.Oid),
						slog.Any("err", err),
					)
				}
				progress := float64(uploadedPartCount) / float64(totalParts) * 100
				err = w.syncCache.CacheLfsSyncFileProgress(ctx, repoPath, pointer.Oid, int(progress))
				if err != nil {
					slog.Error(
						"failed to cache progress",
						slog.Int("workerID", w.id),
						slog.Any("repoPath", repoPath),
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
	err := eg.Wait()
	if err != nil {
		slog.Error(
			"failed to download and upload lfs file",
			slog.Int("workerID", w.id),
			slog.Any("repoPath", repoPath),
			slog.Any("objectKey", objectKey),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to download and upload lfs file: %w", err)
	}

	result, err := w.ossCore.ListObjectParts(ctx, w.config.S3.Bucket, objectKey, uploadID, 0, 0)
	if err != nil {
		slog.Error(
			"failed to list object parts",
			slog.Int("workerID", w.id),
			slog.Any("repoPath", repoPath),
			slog.Any("objectKey", objectKey),
			slog.Any("oid", pointer.Oid),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to list object parts: %w", err)
	}

	if len(result.ObjectParts) != totalParts {
		partNumberMapping := make(map[int]struct{})
		for _, part := range result.ObjectParts {
			partNumberMapping[part.PartNumber] = struct{}{}
		}
		for partNumber := 1; partNumber <= int(totalParts); partNumber++ {
			_, ok := partNumberMapping[partNumber]
			if !ok {
				slog.Error("part number missing", slog.Any("partNumber", partNumber), slog.Any("oid", pointer.Oid))
				err := w.syncCache.DeleteSpecificLfsPartCache(ctx, repoPath, pointer.Oid, partNumber)
				if err != nil {
					slog.Error(
						"failed to delete specific lfs part cache",
						slog.Int("workerID", w.id),
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

	// Sort the parts by part number
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

	_, err = w.ossCore.CompleteMultipartUpload(
		ctx, w.config.S3.Bucket, objectKey, uploadID, parts, minio.PutObjectOptions{
			DisableContentSha256: true,
		},
	)
	if err != nil {
		slog.Error(
			"failed to complete multipart upload",
			slog.Any("workerID", w.id),
			slog.Any("error", err),
			slog.Any("uploadID", uploadID),
			slog.Any("objectKey", objectKey),
			slog.Any("repoPath", repoPath),
		)
		return fmt.Errorf("failed to complete multipart upload, %w", err)
	}

	slog.Info(
		"complete multipart upload",
		slog.Any("workerID", w.id),
		slog.Any("error", err),
		slog.Any("uploadID", uploadID),
		slog.Any("objectKey", objectKey),
		slog.Any("repoPath", repoPath),
	)
	return nil
}

func (w *LfsSyncWorker) downloadAndUploadPartWithRetry(
	ctx context.Context,
	downloadURL, uploadID, objectKey string,
	partNumber int,
	start int64,
	end int64,
) (minio.ObjectPart, error) {
	var (
		part    minio.ObjectPart
		lastErr error
	)
	repoPath := ctx.Value(rk).(string)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info(
			"downloading range",
			slog.Int("workerID", w.id),
			slog.Any("repoPath", repoPath),
			slog.Any("objectKey", objectKey),
			slog.Any("downloadURL", downloadURL),
			slog.Any("partNumber", partNumber),
			slog.Any("offset", start),
			slog.Any("end", end),
		)
		resp, err := w.downloadRange(downloadURL, start, end)
		if err != nil {
			slog.Error(
				"failed to download range",
				slog.Int("workerID", w.id),
				slog.Any("downloadURL", downloadURL),
				slog.Any("partNumber", partNumber),
				slog.Any("start", start),
				slog.Any("end", end),
				slog.Any("attempt", attempt),
				slog.Any("error", err),
			)
			return part, fmt.Errorf("failed to download range: %w", err)
		}

		defer resp.Body.Close()

		slog.Info(
			"uploading range",
			slog.Any("workerID", w.id),
			slog.Any("repoPath", repoPath),
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
			slog.Error(
				"failed to upload range",
				slog.Any("workerID", w.id),
				slog.Any("repoPath", repoPath),
				slog.Any("objectKey", objectKey),
				slog.Any("partNumber", partNumber),
				slog.Any("offset", start),
				slog.Any("end", end),
				slog.Any("attempt", attempt),
				slog.Any("error", err),
			)

			time.Sleep(3 * time.Second)
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

func (w *LfsSyncWorker) downloadRange(
	downloadURL string,
	start, end int64,
) (*http.Response, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
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

	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Set("Range", rangeHeader)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return resp, nil
}

func (w *LfsSyncWorker) CheckIfLFSFileExists(
	ctx context.Context,
	objectKey string,
) (bool, error) {
	_, err := w.ossClient.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w *LfsSyncWorker) GetLFSDownloadURLs(
	ctx context.Context,
	repoCloneURL, branch string,
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

	req, err := http.NewRequest("POST", lfsAPIURL, bytes.NewReader(payload))
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

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get lfs download url, unexpected status code: %d", resp.StatusCode)
	}

	var batchResp types.LFSBatchResponse
	err = json.NewDecoder(resp.Body).Decode(&batchResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	if len(batchResp.Objects) == 0 {
		return nil, fmt.Errorf("no objects returned in batch response")
	}

	for _, obj := range batchResp.Objects {
		resPointers = append(resPointers, &types.Pointer{
			Oid:         obj.Oid,
			Size:        obj.Size,
			DownloadURL: obj.Actions.Download.Href,
		})
	}

	return resPointers, nil
}

func (w *LfsSyncWorker) getRepoLastCommit(
	ctx context.Context,
	namespace, name, branch string,
	repoType types.RepositoryType,
) (*types.Commit, error) {
	commit, err := w.git.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  repoType,
		Ref:       branch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get repo last commit: %w", err)
	}
	return commit, nil
}

func (w *LfsSyncWorker) triggerGitCallback(
	ctx context.Context,
	namespace, name, branch string,
	commit *types.Commit,
	repo *database.Repository,
) error {
	callback, err := w.git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     namespace,
		Name:          name,
		RepoType:      repo.RepositoryType,
		Ref:           branch,
		LeftCommitId:  gitaly.SHA1EmptyTreeID,
		RightCommitId: commit.ID,
		Private:       repo.Private,
	})
	if err != nil {
		return fmt.Errorf("failed to get diff between two commits: %w", err)
	}
	callback.Ref = branch

	//start workflow to handle push request
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
	}

	we, err := workflowClient.ExecuteWorkflow(
		ctx, workflowOptions, workflow.HandlePushWorkflow, callback,
	)
	if err != nil {
		return fmt.Errorf("failed to handle git push callback: %w", err)
	}

	slog.Info(
		"start handle push workflow",
		slog.Any("workerID", w.id),
		slog.String("workflowID", we.GetID()),
		slog.Any("req", callback),
		slog.Any("repoType", repo.RepositoryType),
		slog.Any("repoPath", repo.Path))

	return nil
}

func SplitPointersBySizeAndCount(pointers []*types.Pointer) [][]*types.Pointer {
	var groups [][]*types.Pointer
	var currentGroup []*types.Pointer
	var currentSize int64 = 0

	for _, p := range pointers {
		if currentSize+p.Size > MaxGroupSize || len(currentGroup) >= MaxGroupCount {
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
