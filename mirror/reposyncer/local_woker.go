package reposyncer

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/queue"
)

type LocalMirrorWoker struct {
	mq                 *queue.PriorityQueue
	tasks              chan queue.MirrorTask
	numWorkers         int
	wg                 sync.WaitGroup
	saas               bool
	mirrorStore        *database.MirrorStore
	lfsMetaObjectStore *database.LfsMetaObjectStore
	repoStore          *database.RepoStore
	git                gitserver.GitServer
	config             *config.Config
}

func NewLocalMirrorWoker(config *config.Config, numWorkers int) (*LocalMirrorWoker, error) {
	var err error
	w := &LocalMirrorWoker{}
	w.numWorkers = numWorkers
	w.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	w.mirrorStore = database.NewMirrorStore()
	w.repoStore = database.NewRepoStore()
	w.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	w.saas = config.Saas
	w.config = config
	mq, err := queue.GetPriorityQueueInstance()
	if err != nil {
		return nil, fmt.Errorf("fail to get priority queue: %w", err)
	}
	w.mq = mq
	w.tasks = make(chan queue.MirrorTask)
	w.numWorkers = numWorkers
	return w, nil
}

func (w *LocalMirrorWoker) Run() {
	for i := 1; i <= w.numWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	go w.dispatcher()
	w.wg.Wait()
}

func (w *LocalMirrorWoker) dispatcher() {
	for {
		task := w.mq.PopRepoMirror()
		if task != nil {
			w.tasks <- *task
		}
	}
}

func (w *LocalMirrorWoker) worker(id int) {
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
		slog.Info("start to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id))
		err := w.SyncRepo(context.Background(), task)
		if err != nil {
			slog.Info("fail to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id), slog.String("error", err.Error()))
		}
		slog.Info("finish to mirror", slog.Int64("mirrorId", task.MirrorID), slog.Int("priority", task.Priority.Int()), slog.Int("workerId", id))
	}
}

func (w *LocalMirrorWoker) SyncRepo(ctx context.Context, task queue.MirrorTask) error {
	mirror, err := w.mirrorStore.FindByID(ctx, task.MirrorID)
	if err != nil {
		return fmt.Errorf("failed to get mirror: %v", err)
	}
	mirror.Status = types.MirrorRunning
	mirror.Priority = types.LowMirrorPriority
	err = w.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror status: %v", err)
	}
	if mirror.Repository == nil {
		return fmt.Errorf("mirror repository is nil")
	}
	namespace := strings.Split(mirror.Repository.Path, "/")[0]
	name := strings.Split(mirror.Repository.Path, "/")[1]

	slog.Info("Start to sync mirror repo", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)
	req := gitserver.MirrorSyncReq{
		Namespace:   namespace,
		Name:        name,
		CloneUrl:    mirror.SourceUrl,
		Username:    mirror.Username,
		AccessToken: mirror.AccessToken,
		RepoType:    mirror.Repository.RepositoryType,
	}
	if task.MirrorToken != "" {
		req.MirrorToken = task.MirrorToken
	}
	err = w.git.MirrorSync(ctx, req)

	if err != nil {
		return fmt.Errorf("failed mirror remote repo in git server: %v", err)
	}
	slog.Info("Mirror remote repo in git server successfully", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)

	resp, err := w.git.GetRepo(ctx, gitserver.GetRepoReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  mirror.Repository.RepositoryType,
	})
	if err != nil {
		return fmt.Errorf("failed to get repo default branch: %w", err)
	}
	parts := strings.Split(string(resp.DefaultBranch), "/")
	branch := parts[len(parts)-1]

	mirror.Repository.DefaultBranch = branch
	mirror.Repository.SyncStatus = types.SyncStatusInProgress
	_, err = w.repoStore.UpdateRepo(ctx, *mirror.Repository)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status to in progress: %w", err)
	}
	slog.Info("Update repo default branch successfully", slog.Any("repo_type", mirror.Repository.RepositoryType), slog.Any("namespace", namespace), slog.Any("name", name))
	slog.Info("Start to sync lfs files", "repo_type", mirror.Repository.RepositoryType, "namespace", namespace, "name", name)
	lfsFileCount, err := w.generateLfsMetaObjects(ctx, mirror)
	if err != nil {
		mirror.Status = types.MirrorIncomplete
		mirror.LastMessage = err.Error()
		err = w.mirrorStore.Update(ctx, mirror)
		if err != nil {
			return fmt.Errorf("failed to update mirror: %w", err)
		}

		mirror.Repository.SyncStatus = types.SyncStatusFailed
		_, err = w.repoStore.UpdateRepo(ctx, *mirror.Repository)
		if err != nil {
			return fmt.Errorf("failed to update repo sync status to failed: %w", err)
		}
		return fmt.Errorf("failed to sync lfs files: %v", err)
	}
	if lfsFileCount > 0 {
		mirror.Status = types.MirrorRepoSynced
		w.mq.PushLfsMirror(&queue.MirrorTask{
			MirrorID:    mirror.ID,
			Priority:    queue.Priority(mirror.Priority),
			CreatedAt:   mirror.CreatedAt.Unix(),
			MirrorToken: task.MirrorToken,
		})
	} else {
		mirror.Status = types.MirrorFinished
	}

	err = w.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror: %w", err)
	}

	return nil
}

func (c *LocalMirrorWoker) generateLfsMetaObjects(ctx context.Context, mirror *database.Mirror) (int, error) {
	var lfsMetaObjects []database.LfsMetaObject
	namespace := strings.Split(mirror.Repository.Path, "/")[0]
	name := strings.Split(mirror.Repository.Path, "/")[1]
	branches, err := c.git.GetRepoBranches(ctx, gitserver.GetBranchesReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  mirror.Repository.RepositoryType,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get repo branches: %v", err)
	}
	for _, branch := range branches {
		lfsPointers, err := c.getAllLfsPointersByRef(ctx, mirror.Repository.RepositoryType, namespace, name, branch.Name)
		if err != nil {
			return 0, fmt.Errorf("failed to get all lfs pointers: %v", err)
		}
		for _, lfsPointer := range lfsPointers {
			lfsMetaObjects = append(lfsMetaObjects, database.LfsMetaObject{
				Size:         lfsPointer.FileSize,
				Oid:          lfsPointer.FileOid,
				RepositoryID: mirror.Repository.ID,
				Existing:     true,
			})
		}
	}
	lfsMetaObjects = removeDuplicateLfsMetaObject(lfsMetaObjects)

	if len(lfsMetaObjects) > 0 {
		err = c.lfsMetaObjectStore.BulkUpdateOrCreate(ctx, lfsMetaObjects)
		if err != nil {
			return 0, fmt.Errorf("failed to bulk update or create lfs meta objects: %v", err)
		}
	}

	return len(lfsMetaObjects), nil
}

func (c *LocalMirrorWoker) getAllLfsPointersByRef(ctx context.Context, RepoType types.RepositoryType, namespace, name, ref string) ([]*types.LFSPointer, error) {
	return c.git.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  RepoType,
	})
}

func removeDuplicateLfsMetaObject(objects []database.LfsMetaObject) []database.LfsMetaObject {
	seen := make(map[string]bool)
	uniqueObjects := []database.LfsMetaObject{}

	for _, obj := range objects {
		key := obj.Oid + "_" + strconv.Itoa(int(obj.RepositoryID))
		if !seen[key] {
			uniqueObjects = append(uniqueObjects, obj)
			seen[key] = true
		}
	}

	return uniqueObjects
}
