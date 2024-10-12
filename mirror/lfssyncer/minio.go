package lfssyncer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/queue"
)

type MinioLFSSyncWorker struct {
	mq                 *queue.PriorityQueue
	tasks              chan queue.MirrorTask
	wg                 sync.WaitGroup
	mirrorStore        *database.MirrorStore
	repoStore          *database.RepoStore
	lfsMetaObjectStore *database.LfsMetaObjectStore
	s3Client           *minio.Client
	config             *config.Config
	numWorkers         int
}

func NewMinioLFSSyncWorker(config *config.Config, numWorkers int) (*MinioLFSSyncWorker, error) {
	var err error
	w := &MinioLFSSyncWorker{}
	w.numWorkers = numWorkers
	w.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	w.mirrorStore = database.NewMirrorStore()
	w.repoStore = database.NewRepoStore()
	w.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	w.config = config
	mq, err := queue.GetPriorityQueueInstance()
	if err != nil {
		return nil, fmt.Errorf("fail to get priority queue: %w", err)
	}
	w.mq = mq
	w.tasks = make(chan queue.MirrorTask)
	return w, nil
}

func (w *MinioLFSSyncWorker) Run() {
	for i := 1; i <= w.numWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	go w.dispatcher()
	w.wg.Wait()
}

func (w *MinioLFSSyncWorker) dispatcher() {
	for {
		task := w.mq.PopLfsMirror()
		if task != nil {
			w.tasks <- *task
		}
	}
}

func (w *MinioLFSSyncWorker) worker(id int) {
	defer w.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			w.wg.Add(1)
			go w.worker(id)
			slog.Info("worker ecovered from panic ", slog.Int("workerId", id))
		}
	}()
	slog.Info("worker start", slog.Int("workerId", id))
	for {
		task := <-w.tasks
		ctx := context.Background()
		mirror, err := w.mirrorStore.FindByID(ctx, task.MirrorID)
		if err != nil {
			slog.Error("fail to get mirror", slog.Int("workerId", id), slog.String("error", err.Error()))
			continue
		}
		repo, err := w.repoStore.FindById(ctx, mirror.RepositoryID)
		if err != nil {
			slog.Error("fail to get repository", slog.Int("workerId", id), slog.String("error", err.Error()))
			continue
		}
		err = w.SyncLfs(ctx, id, mirror)
		if err != nil {
			repo.SyncStatus = types.SyncStatusFailed
			_, repoErr := w.repoStore.UpdateRepo(ctx, *repo)
			if repoErr != nil {
				slog.Error("fail to update repo sync status to failed: %w", slog.Any("error", err))
			}
			slog.Error("fail to sync lfs", slog.Int("workerId", id), slog.String("error", err.Error()))
			continue
		}

		repo.SyncStatus = types.SyncStatusCompleted
		_, err = w.repoStore.UpdateRepo(ctx, *repo)
		if err != nil {
			slog.Error("fail to update repo sync status to complete: %w", slog.Any("error", err))
		}
	}
}

func (w *MinioLFSSyncWorker) SyncLfs(ctx context.Context, workerId int, mirror *database.Mirror) error {
	var pointers []*types.Pointer
	lfsMetaObjects, err := w.lfsMetaObjectStore.FindByRepoID(ctx, mirror.Repository.ID)
	if err != nil {
		slog.Error("fail to get lfs meta objects", slog.Int("workerId", workerId), slog.String("error", err.Error()))
		return fmt.Errorf("fail to get lfs meta objects: %w", err)
	}
	for _, lfsMetaObject := range lfsMetaObjects {
		pointers = append(pointers, &types.Pointer{
			Oid:  lfsMetaObject.Oid,
			Size: lfsMetaObject.Size,
		})
	}

	pointers, err = w.GetLFSDownloadURLs(ctx, mirror, pointers)
	if err != nil {
		return fmt.Errorf("fail to get LFS download URL: %w", err)
	}
	err = w.DownloadAndUploadLFSFiles(ctx, mirror, pointers)
	if err != nil {
		return fmt.Errorf("fail to download and upload LFS files: %w", err)
	}
	return nil
}

func (w *MinioLFSSyncWorker) GetLFSDownloadURLs(ctx context.Context, mirror *database.Mirror, pointers []*types.Pointer) ([]*types.Pointer, error) {
	var resPointers []*types.Pointer
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

	lfsAPIURL := mirror.SourceUrl + "/info/lfs/objects/batch"

	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return resPointers, fmt.Errorf("failed to marshal request payload: %v", err)
	}

	req, err := http.NewRequest("POST", lfsAPIURL, bytes.NewReader(payload))
	if err != nil {
		return resPointers, fmt.Errorf("failed to create LFS batch request: %v", err)
	}

	parsedURL, err := url.Parse(lfsAPIURL)
	if err != nil {
		return resPointers, fmt.Errorf("failed to parse LFS API URL: %v", err)
	}

	req.Header.Set("Host", parsedURL.Host)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json; charset=utf-8")
	req.Header.Set("User-Agent", "git-lfs/3.5.1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resPointers, fmt.Errorf("failed to send LFS batch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resPointers, fmt.Errorf("failed to get LFS download URL, status code: %d", resp.StatusCode)
	}

	var batchResponse types.LFSBatchResponse
	err = json.NewDecoder(resp.Body).Decode(&batchResponse)
	if err != nil {
		return resPointers, fmt.Errorf("failed to decode LFS batch response: %v", err)
	}

	if len(batchResponse.Objects) == 0 {
		return resPointers, fmt.Errorf("no objects found in LFS batch response")
	}
	for _, object := range batchResponse.Objects {
		resPointers = append(resPointers, &types.Pointer{
			Oid:         object.Oid,
			Size:        object.Size,
			DownloadURL: object.Actions.Download.Href,
		})
	}

	return resPointers, nil
}

func (w *MinioLFSSyncWorker) DownloadAndUploadLFSFiles(ctx context.Context, mirror *database.Mirror, pointers []*types.Pointer) error {
	var finishedLFSFileCount int
	lfsFilesCount := len(pointers)
	for _, pointer := range pointers {
		objectKey := filepath.Join("lfs", pointer.RelativePath())
		fileInfo, err := w.s3Client.StatObject(ctx, w.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
		if err != nil && err.Error() != "The specified key does not exist." {
			slog.Error("failed to check if LFS file exists", slog.Any("error", err))
			continue
		}
		if (err != nil && err.Error() != "The specified key does not exist.") || fileInfo.Size != pointer.Size {
			err = w.DownloadAndUploadLFSFile(ctx, mirror, pointer)
			if err != nil {
				slog.Error("failed to download and upload LFS file", slog.Any("error", err))
			}
		}

		lfsMetaObject := database.LfsMetaObject{
			Size:         pointer.Size,
			Oid:          pointer.Oid,
			RepositoryID: mirror.Repository.ID,
			Existing:     true,
		}
		_, err = w.lfsMetaObjectStore.UpdateOrCreate(ctx, lfsMetaObject)
		if err != nil {
			slog.Error("failed to update or create LFS meta object", slog.Any("error", err))
			return fmt.Errorf("failed to update or create LFS meta object: %w", err)
		}
		slog.Info("finish to download and upload LFS file", slog.Any("objectKey", objectKey))
		finishedLFSFileCount += 1
		mirror.Progress = int8(finishedLFSFileCount * 100 / lfsFilesCount)
		err = w.mirrorStore.Update(ctx, mirror)
		if err != nil {
			return fmt.Errorf("failed to update mirror progress: %w", err)
		}
	}
	mirror.Status = types.MirrorFinished
	err := w.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror status: %w", err)
	}
	return nil
}

func (w *MinioLFSSyncWorker) DownloadAndUploadLFSFile(ctx context.Context, mirror *database.Mirror, pointer *types.Pointer) error {
	objectKey := filepath.Join("lfs", pointer.RelativePath())
	slog.Info("downloading LFS file from", slog.Any("url", pointer.DownloadURL))

	req, err := http.NewRequest("GET", pointer.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create downlaod request: %w", err)
	}

	parsedURL, err := url.Parse(pointer.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to parse LFS API URL: %v", err)
	}

	req.Header.Set("Host", parsedURL.Host)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json; charset=utf-8")
	req.Header.Set("User-Agent", "git-lfs/3.5.1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download LFS file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download LFS file: %s", resp.Status)
	}
	slog.Info("uploading LFS file", slog.Any("object_key", objectKey))
	uploadInfo, err := w.s3Client.PutObject(ctx, w.config.S3.Bucket, objectKey, resp.Body, resp.ContentLength, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload to Minio: %w", err)
	}

	if uploadInfo.Size != pointer.Size {
		return fmt.Errorf("uploaded file size does not match expected size: %d != %d", uploadInfo.Size, pointer.Size)
	}

	return nil
}
