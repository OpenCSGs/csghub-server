package mirror

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/mirror/queue"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MirrorService struct {
	mq                 *queue.PriorityQueue
	tasks              chan queue.MirrorTask
	numWorkers         int
	wg                 sync.WaitGroup
	tokenStore         *database.GitServerAccessTokenStore
	saas               bool
	mirrorStore        *database.MirrorStore
	repoStore          *database.RepoStore
	modelStore         *database.ModelStore
	datasetStore       *database.DatasetStore
	codeStore          *database.CodeStore
	mirrorSourceStore  *database.MirrorSourceStore
	namespaceStore     *database.NamespaceStore
	lfsMetaObjectStore *database.LfsMetaObjectStore
	git                gitserver.GitServer
	s3Client           *minio.Client
	lfsBucket          string
	config             *config.Config
}

func NewMirrorService(config *config.Config, numWorkers int) (*MirrorService, error) {
	var err error
	s := &MirrorService{}
	s.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	s.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	s.lfsBucket = config.S3.Bucket
	s.modelStore = database.NewModelStore()
	s.datasetStore = database.NewDatasetStore()
	s.codeStore = database.NewCodeStore()
	s.repoStore = database.NewRepoStore()
	s.mirrorStore = database.NewMirrorStore()
	s.tokenStore = database.NewGitServerAccessTokenStore()
	s.mirrorSourceStore = database.NewMirrorSourceStore()
	s.namespaceStore = database.NewNamespaceStore()
	s.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	s.saas = config.Saas
	s.config = config
	s.mq = queue.GetPriorityQueueInstance()
	s.tasks = make(chan queue.MirrorTask)
	s.numWorkers = numWorkers
	return s, nil
}

func (ms *MirrorService) Enqueue(task *queue.MirrorTask) {
	ms.mq.Push(task)
}

func (ms *MirrorService) Start() {
	for i := 1; i <= ms.numWorkers; i++ {
		ms.wg.Add(1)
		go ms.worker(i)
	}
	go ms.dispatcher()
	ms.wg.Wait()
}

func (ms *MirrorService) EnqueueMirrorTasks() {
	mirrorStore := database.NewMirrorStore()
	mirrors, err := mirrorStore.ToSync(context.Background())
	if err != nil {
		slog.Error("fail to get mirror to sync", slog.String("error", err.Error()))
		return
	}

	for _, mirror := range mirrors {
		ms.mq.Push(&queue.MirrorTask{MirrorID: mirror.ID, Priority: queue.Priority(mirror.Priority)})
		mirror.Status = types.MirrorWaiting
		err = mirrorStore.Update(context.Background(), &mirror)
		if err != nil {
			slog.Error("fail to update mirror status", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
			continue
		}
	}
}

func (ms *MirrorService) worker(id int) {
	defer ms.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ms.wg.Add(1)
			go ms.worker(id)
			slog.Info("worker ecovered from panic ", slog.Int("workerId", id))
		}
	}()
	slog.Info("worker start", slog.Int("workerId", id))
	for {
		task := <-ms.tasks
		slog.Info("start to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id))
		err := ms.Mirror(context.Background(), task.MirrorID)
		if err != nil {
			slog.Info("fail to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id), slog.String("error", err.Error()))
		}
		slog.Info("finish to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id))
	}
}

func (ms *MirrorService) dispatcher() {
	for {
		task := ms.mq.Pop()
		if task != nil {
			ms.tasks <- *task
		}
	}
}

func (c *MirrorService) Mirror(ctx context.Context, mirrorID int64) error {
	mirror, err := c.mirrorStore.FindByID(ctx, mirrorID)
	if err != nil {
		return fmt.Errorf("failed to get mirror: %v", err)
	}
	mirror.Status = types.MirrorRunning
	err = c.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror status: %v", err)
	}
	if mirror.Repository == nil {
		return fmt.Errorf("mirror repository is nil")
	}
	namespace := strings.Split(mirror.Repository.Path, "/")[0]
	name := strings.Split(mirror.Repository.Path, "/")[1]

	slog.Info("Start to sync mirror", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)
	err = c.git.MirrorSync(ctx, gitserver.MirrorSyncReq{
		Namespace:   namespace,
		Name:        name,
		CloneUrl:    mirror.SourceUrl,
		Username:    mirror.Username,
		AccessToken: mirror.AccessToken,
		RepoType:    mirror.Repository.RepositoryType,
	})

	if err != nil {
		return fmt.Errorf("failed mirror remote repo in git server: %v", err)
	}
	slog.Info("Mirror remote repo in git server successfully", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)
	slog.Info("Start to sync lfs files", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)
	err = c.syncLfsFiles(ctx, mirror)
	if err != nil {
		mirror.Status = types.MirrorIncomplete
		mirror.LastMessage = err.Error()
		err = c.mirrorStore.Update(ctx, mirror)
		if err != nil {
			return fmt.Errorf("failed to update mirror: %w", err)
		}
		return fmt.Errorf("failed to sync lfs files: %v", err)
	}
	mirror.NextExecutionTimestamp = time.Now().Add(24 * time.Hour)
	mirror.Status = types.MirrorFinished
	mirror.Priority = types.LowMirrorPriority
	err = c.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror: %w", err)
	}

	return nil
}

func (c *MirrorService) syncLfsFiles(ctx context.Context, mirror *database.Mirror) error {
	var pointers []*types.Pointer
	namespace := strings.Split(mirror.Repository.Path, "/")[0]
	name := strings.Split(mirror.Repository.Path, "/")[1]
	branches, err := c.git.GetRepoBranches(ctx, gitserver.GetBranchesReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  mirror.Repository.RepositoryType,
	})
	if err != nil {
		return fmt.Errorf("failed to get repo branches: %v", err)
	}
	for _, branch := range branches {
		lfsPointers, err := c.getAllLfsPointersByRef(ctx, mirror.Repository.RepositoryType, namespace, name, branch.Name)
		if err != nil {
			return fmt.Errorf("failed to get all lfs pointers: %v", err)
		}
		for _, lfsPointer := range lfsPointers {
			pointers = append(pointers, &types.Pointer{
				Oid:  lfsPointer.FileOid,
				Size: lfsPointer.FileSize,
			})
		}
	}

	pointers, err = c.GetLFSDownloadURLs(ctx, mirror, pointers)
	if err != nil {
		return fmt.Errorf("failed to get LFS download URLs: %v", err)
	}
	err = c.DownloadAndUploadLFSFiles(ctx, mirror, pointers)
	if err != nil {
		return err
	}

	return nil
}

func (c *MirrorService) getAllLfsPointersByRef(ctx context.Context, RepoType types.RepositoryType, namespace, name, ref string) ([]*types.LFSPointer, error) {
	return c.git.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  RepoType,
	})
}

func (c *MirrorService) GetLFSDownloadURLs(ctx context.Context, mirror *database.Mirror, pointers []*types.Pointer) ([]*types.Pointer, error) {
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

	lfsAPIURL := mirror.SourceUrl + "/info/lfs/objects/batch"

	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return resPointers, fmt.Errorf("failed to marshal request payload: %v", err)
	}

	resp, err := http.Post(lfsAPIURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return resPointers, fmt.Errorf("failed to get LFS download URL: %v", err)
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

func (c *MirrorService) DownloadAndUploadLFSFiles(ctx context.Context, mirror *database.Mirror, pointers []*types.Pointer) error {
	var finishedLFSFileCount int
	lfsFilesCount := len(pointers)
	for _, pointer := range pointers {
		objectKey := filepath.Join("lfs", pointer.RelativePath())
		fileInfo, err := c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
		if err != nil && err.Error() != "The specified key does not exist." {
			slog.Error("failed to check if LFS file exists", slog.Any("error", err))
			continue
		}
		if (err != nil && err.Error() != "The specified key does not exist.") || fileInfo.Size != pointer.Size {
			err = c.DownloadAndUploadLFSFile(ctx, mirror, pointer)
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
		_, err = c.lfsMetaObjectStore.UpdateOrCreate(ctx, lfsMetaObject)
		if err != nil {
			slog.Error("failed to update or create LFS meta object", slog.Any("error", err))
			return fmt.Errorf("failed to update or create LFS meta object: %w", err)
		}
		finishedLFSFileCount += 1
		mirror.Progress = int8(finishedLFSFileCount * 100 / lfsFilesCount)
		err = c.mirrorStore.Update(ctx, mirror)
		if err != nil {
			return fmt.Errorf("failed to update mirror progress: %w", err)
		}
	}
	return nil
}

func (c *MirrorService) DownloadAndUploadLFSFile(ctx context.Context, mirror *database.Mirror, pointer *types.Pointer) error {
	objectKey := filepath.Join("lfs", pointer.RelativePath())
	slog.Info("downloading LFS file from", slog.Any("url", pointer.DownloadURL))
	resp, err := http.Get(pointer.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download LFS file: %s", resp.Status)
	}
	slog.Info("uploading LFS file", slog.Any("object_key", objectKey))
	uploadInfo, err := c.s3Client.PutObject(ctx, c.config.S3.Bucket, objectKey, resp.Body, resp.ContentLength, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload to Minio: %w", err)
	}

	if uploadInfo.Size != pointer.Size {
		return fmt.Errorf("uploaded file size does not match expected size: %d != %d", uploadInfo.Size, pointer.Size)
	}

	return nil
}
