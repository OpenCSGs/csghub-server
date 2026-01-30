package reposyncer

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/client"
	"golang.org/x/time/rate"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/hook"
)

const specificErrorMsg = "rate limit"

var MirrorStatusToMessageTypeMapping = map[types.MirrorTaskStatus]string{
	types.MirrorRepoSyncStart:   "repo_sync_start",
	types.MirrorLfsSyncStart:    "lfs_sync_start",
	types.MirrorLfsSyncFailed:   "sync_failed",
	types.MirrorLfsSyncFinished: "sync_finished",
	types.MirrorRepoTooLarge:    "repo_too_large",
}

var expectedMirrorTaskStatus = []types.MirrorTaskStatus{
	types.MirrorQueued,
}

type RepoSyncWorker struct {
	tasks                  chan database.MirrorTask
	numWorkers             int
	wg                     sync.WaitGroup
	saas                   bool
	mirrorStore            database.MirrorStore
	mirrorTaskStore        database.MirrorTaskStore
	lfsMetaObjectStore     database.LfsMetaObjectStore
	repoStore              database.RepoStore
	syncClientSettingStore database.SyncClientSettingStore
	git                    gitserver.GitServer
	config                 *config.Config
	ratelimiter            *rate.Limiter
	msgSender              hook.MessageSender
	httpClient             *http.Client
}

func NewRepoSyncWorker(config *config.Config, numWorkers int) (*RepoSyncWorker, error) {
	var err error
	w := &RepoSyncWorker{}
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
	w.syncClientSettingStore = database.NewSyncClientSettingStore()
	w.mirrorTaskStore = database.NewMirrorTaskStore()
	w.saas = config.Saas
	w.config = config
	w.tasks = make(chan database.MirrorTask)
	w.numWorkers = numWorkers
	w.ratelimiter = rate.NewLimiter(
		rate.Limit(config.Mirror.RateLimit),
		config.Mirror.RateBucketCapacity,
	)
	msgSender := hook.NewMessageSender(
		fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken),
		rpc.WithJSONHeader(),
	)
	w.msgSender = msgSender
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.MultiSync.HTTPInsecureSkipVerify},
	}
	w.httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: tr,
	}
	return w, nil
}

func (w *RepoSyncWorker) Run() {
	for i := 0; i < w.numWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	go w.diapatcher()
	w.wg.Wait()
	close(w.tasks)
}

func (w *RepoSyncWorker) diapatcher() {
	for {
		ctx := context.Background()
		task, err := w.mirrorTaskStore.GetHighestPriorityByTaskStatus(ctx, expectedMirrorTaskStatus)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.Info("no tasks to dispatch, sleep 5s")
				time.Sleep(5 * time.Second)
				continue
			}
			slog.Error("failed to get task from db", slog.Any("error", err))
			time.Sleep(5 * time.Second)
			continue
		}
		w.tasks <- task
	}
}

func (w *RepoSyncWorker) worker(workerID int) {
	defer w.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			w.wg.Add(1)
			go w.worker(workerID)
			slog.Info(
				"worker recoverd from panic",
				slog.Any("workerID", workerID),
				slog.Any("panic", r),
			)
		} else {
			slog.Info("worker done", slog.Any("workerID", workerID))
		}
	}()
	slog.Info("worker start", slog.Any("workerID", workerID))
	for {
		err := w.ratelimiter.Wait(context.Background())
		if err != nil {
			slog.Error("Error waiting for rate limiter:", slog.Any("error", err))
			continue
		}

		task := <-w.tasks

		if task.ID != 0 {
			ctx := context.Background()
			w.handleTask(ctx, &task, workerID)
		}
	}
}

func (w *RepoSyncWorker) handleTask(
	ctx context.Context,
	mt *database.MirrorTask,
	workerID int,
) {
	var statusAction string
	mirror, err := w.mirrorStore.FindByID(ctx, mt.MirrorID)
	if err != nil {
		slog.Error("failed to get mirror", slog.Any("error", err))
		return
	}

	slog.Info(
		"start to mirror",
		slog.Int64("mirrorId", mirror.ID),
		slog.Any("priority", mirror.Priority),
		slog.Int("workerID", workerID),
		slog.Any("repoPath", mirror.Repository.Path),
	)

	mt, err = w.SyncRepo(ctx, mirror, mt)

	if err != nil {
		mt.ErrorMessage = err.Error()
		statusAction = string(database.MirrorFail)
		if strings.Contains(err.Error(), specificErrorMsg) {
			mt.RetryCount += 1
			if mt.RetryCount > w.config.Mirror.MaxRetryCount {
				statusAction = string(database.MirrorFatal)
			} else {
				statusAction = string(database.MirrorRetry)
			}
		}
		slog.Error("failed to sync repo", slog.Any("error", err))
		sendErr := w.sendMessage(ctx, mt.Mirror, types.MirrorRepoSyncFailed)
		if sendErr != nil {
			slog.Error("failed to send notice message", slog.Any("error", sendErr))
		}
	} else {
		if mt.Progress == 100 {
			statusAction = string(database.MirrorNoLfsToSync)
		} else {
			statusAction = string(database.MirrorSuccess)
		}
	}

	mtFSM := database.NewMirrorTaskWithFSM(mt)
	canContinue := mtFSM.SubmitEvent(ctx, statusAction)
	if !canContinue {
		slog.Error(
			"failed to transition to next status",
			slog.Any("before status", mt.Status),
			slog.Any("action", statusAction),
		)
		return
	}
	mt.Status = types.MirrorTaskStatus(mtFSM.Current())
	repoSyncStatus := common.MirrorTaskStatusToRepoStatus(mt.Status)
	_, err = w.mirrorTaskStore.UpdateStatusAndRepoSyncStatus(ctx, *mt, repoSyncStatus)
	if err != nil {
		slog.Error("failed to update mirror task status and repository status", slog.Any("error", err))
	}
}

func (w *RepoSyncWorker) SyncRepo(
	ctx context.Context,
	mirror *database.Mirror,
	mt *database.MirrorTask,
) (*database.MirrorTask, error) {
	var commitBefore *types.Commit

	// Check if repository is not present
	if mirror.Repository == nil {
		return mt, fmt.Errorf("mirror repository is nil")
	}

	// Send Message
	err := w.sendMessage(ctx, mirror, types.MirrorRepoSyncStart)
	if err != nil {
		slog.Error("failed to send notice message", slog.Any("error", err))
	}

	namespace, name, err := common.GetNamespaceAndNameFromPath(mirror.Repository.Path)
	if err != nil {
		return mt, fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
	}

	// Check if the repository already exists, if not, create it
	err = w.ensureRepoExists(ctx, namespace, name, mirror.Repository.DefaultBranch, mirror.Repository.RepositoryType)
	if err != nil {
		return mt, fmt.Errorf("failed to ensure repository exists: %w", err)
	}

	// Get before last commit id
	commitBefore, _ = w.getRepoLastCommit(
		ctx,
		namespace,
		name,
		mirror.Repository.DefaultBranch,
		mirror.Repository.RepositoryType,
	)

	if commitBefore != nil {
		mt.BeforeLastCommitID = commitBefore.ID
	}

	slog.Info(
		"Start to sync mirror repo",
		slog.Any("repo_type", mirror.Repository.RepositoryType),
		slog.Any("namespace", namespace),
		slog.Any("name", name),
	)

	req := gitserver.MirrorSyncReq{
		Namespace:   namespace,
		Name:        name,
		CloneUrl:    mirror.SourceUrl,
		Username:    mirror.Username,
		AccessToken: mirror.AccessToken,
		RepoType:    mirror.Repository.RepositoryType,
	}
	if mirror.Repository.IsOpenCSGRepo() && !w.config.Saas {
		syncClientSetting, err := w.syncClientSettingStore.First(ctx)
		if err != nil {
			return mt, fmt.Errorf("failed to find sync client setting, error: %w", err)
		}
		req.MirrorToken = syncClientSetting.Token
	}

	if err := w.checkSourceURL(ctx, mirror.SourceUrl); err != nil {
		return mt, err
	}

	err = w.git.MirrorSync(ctx, req)
	if err != nil {
		return mt, fmt.Errorf("failed to sync mirror repo, error: %w", err)
	}

	slog.Info(
		"Mirror remote repo in git server successfully",
		slog.Any("repo_type", mirror.Repository.RepositoryType),
		slog.Any("namespace", namespace),
		slog.Any("name", name),
	)

	resp, err := w.git.GetRepo(ctx, gitserver.GetRepoReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  mirror.Repository.RepositoryType,
	})
	if err != nil {
		return mt, fmt.Errorf("failed to get repo, error: %w", err)
	}

	parts := strings.Split(string(resp.DefaultBranch), "/")
	branch := parts[len(parts)-1]

	lfsFileCount, err := w.handleLfsFiles(ctx, mirror, req.MirrorToken)
	if err != nil {
		return mt, fmt.Errorf("failed to handle lfs files, error: %w", err)
	}

	// Update mirror last updated at
	mirror.LastUpdatedAt = time.Now()
	// Update repository informations
	mirror.Repository.DefaultBranch = branch
	err = w.mirrorStore.UpdateMirrorAndRepository(ctx, mirror, mirror.Repository)
	if err != nil {
		return mt, fmt.Errorf("failed to update mirror and repository: %w", err)
	}

	slog.Info(
		"Update repo default branch successfully",
		slog.Any("repo_type", mirror.Repository.RepositoryType),
		slog.Any("namespace", namespace),
		slog.Any("name", name),
	)

	// Get repo last commit
	commit, err := w.getRepoLastCommit(ctx, namespace, name, branch, mirror.Repository.RepositoryType)
	if err != nil {
		return mt, fmt.Errorf("failed to get repo last commit: %w", err)
	}

	if lfsFileCount == 0 {
		mt.Progress = 100
	}

	if lfsFileCount > 0 && commitBefore.ID != "" {
		// Point HEAD to old commit
		slog.Info(
			"Point HEAD to old commit",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)

		err = w.git.UpdateRef(ctx, gitserver.UpdateRefReq{
			Namespace:   namespace,
			Name:        name,
			Ref:         fmt.Sprintf("refs/heads/%s", branch),
			RepoType:    mirror.Repository.RepositoryType,
			NewObjectId: commitBefore.ID,
		})
		if err != nil {
			return mt, fmt.Errorf("failed to point HEAD to old commit: %w", err)
		}
		slog.Info(
			"Point HEAD to old commit successfully",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)
	}

	mt.AfterLastCommitID = commit.ID

	if commit.ID == commitBefore.ID {
		slog.Info(
			"sync repo successfully, no changes detected.",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)
		return mt, nil
	}

	if lfsFileCount == 0 {
		// Trigger git callback
		err = w.triggerGitCallback(ctx, namespace, name, branch, commit, mirror)
		if err != nil {
			return mt, fmt.Errorf("failed to trigger git callback: %w", err)
		}
	}

	return mt, nil
}

func (w *RepoSyncWorker) generateLfsMetaObjects(
	ctx context.Context,
	mirror *database.Mirror,
) (int, error) {
	var (
		lfsMetaObjects []database.LfsMetaObject
		lfsObjectsSize int64
	)
	namespace, name, err := common.GetNamespaceAndNameFromPath(mirror.Repository.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to get namespace and name from path: %w", err)
	}
	branches, err := w.git.GetRepoBranches(ctx, gitserver.GetBranchesReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  mirror.Repository.RepositoryType,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get repo branches: %w", err)
	}
	for _, branch := range branches {
		lfsPointers, err := w.getAllLfsPointersByRef(
			ctx,
			mirror.Repository.RepositoryType,
			namespace,
			name,
			branch.Name,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to get all lfs pointers: %w", err)
		}

		for _, lfsPointer := range lfsPointers {
			lfsMetaObjects = append(lfsMetaObjects, database.LfsMetaObject{
				Size:         lfsPointer.FileSize,
				Oid:          lfsPointer.FileOid,
				RepositoryID: mirror.Repository.ID,
				Existing:     false,
			})
			lfsObjectsSize += lfsPointer.FileSize
		}
	}
	mirror.Repository.LFSObjectsSize = lfsObjectsSize
	lfsMetaObjects = removeDuplicateLfsMetaObject(lfsMetaObjects)

	if len(lfsMetaObjects) > 0 {
		err = w.lfsMetaObjectStore.BulkUpdateOrCreate(ctx, mirror.Repository.ID, lfsMetaObjects)
		if err != nil {
			return 0, fmt.Errorf("failed to bulk update or create lfs meta objects: %w", err)
		}
	}

	return len(lfsMetaObjects), nil
}

func (w *RepoSyncWorker) getAllLfsPointersByRef(
	ctx context.Context,
	RepoType types.RepositoryType,
	namespace, name, ref string,
) ([]*types.LFSPointer, error) {
	return w.git.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  RepoType,
	})
}

func (w *RepoSyncWorker) ensureRepoExists(
	ctx context.Context, namespace, name, branch string,
	repoType types.RepositoryType,
) error {
	exists, err := w.git.RepositoryExists(ctx, gitserver.CheckRepoReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  repoType,
	})
	if err != nil {
		return fmt.Errorf("failed to check repo existence: %w", err)
	}
	if !exists {
		_, err := w.git.CreateRepo(ctx, gitserver.CreateRepoReq{
			Namespace:     namespace,
			Name:          name,
			RepoType:      repoType,
			DefaultBranch: branch,
		})
		if err != nil {
			return fmt.Errorf("failed to create repo: %w", err)
		}
	}
	return nil
}

func (w *RepoSyncWorker) getRepoLastCommit(
	ctx context.Context, namespace, name, branch string,
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

func (w *RepoSyncWorker) triggerGitCallback(
	ctx context.Context, namespace, name, branch string,
	commit *types.Commit,
	mirror *database.Mirror,
) error {
	callback, err := w.git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     namespace,
		Name:          name,
		RepoType:      mirror.Repository.RepositoryType,
		Ref:           branch,
		LeftCommitId:  gitaly.SHA1EmptyTreeID,
		RightCommitId: commit.ID,
		Private:       mirror.Repository.Private,
	})
	if err != nil {
		return fmt.Errorf("failed to get diff between two commits: %w", err)
	}
	callback.Ref = branch

	//start workflow to handle push request
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
		ID:        fmt.Sprintf("mirror-repo-%s-%s-%s-%s", mirror.Repository.RepositoryType, namespace, name, commit.ID),
	}

	we, err := workflowClient.ExecuteWorkflow(
		ctx, workflowOptions, workflow.HandlePushWorkflow, callback,
	)
	if err != nil {
		return fmt.Errorf("failed to handle git push callback: %w", err)
	}

	slog.Info(
		"start handle push workflow",
		slog.String("workflow_id", we.GetID()),
		slog.Any("req", callback),
	)

	return nil
}

func (w *RepoSyncWorker) handleLfsFiles(
	ctx context.Context,
	mirror *database.Mirror,
	token string,
) (int, error) {
	lfsFileCount, err := w.generateLfsMetaObjects(ctx, mirror)
	if err != nil {
		uErr := w.updateMirrorAndRepositoryStatus(ctx, mirror, mirror.Repository, types.MirrorRepoSyncFailed, types.SyncStatusFailed)
		if uErr != nil {
			return lfsFileCount, uErr
		}
		return lfsFileCount, fmt.Errorf("failed to generate lfs meta objects: %w", err)
	}
	return lfsFileCount, nil
}

func (w *RepoSyncWorker) updateMirrorAndRepositoryStatus(
	ctx context.Context,
	mirror *database.Mirror,
	repository *database.Repository,
	mirrorStatus types.MirrorTaskStatus,
	repositoryStatus types.RepositorySyncStatus,
) error {
	mirror.SetStatus(mirrorStatus)
	mirror.Repository.SetSyncStatus(repositoryStatus)
	err := w.mirrorStore.UpdateMirrorAndRepository(ctx, mirror, repository)
	if err != nil {
		return fmt.Errorf("failed to update mirror and repository: %w", err)
	}
	return nil
}

func (w *RepoSyncWorker) sendMessage(
	ctx context.Context,
	mirror *database.Mirror,
	status types.MirrorTaskStatus,
) error {
	statusToSend := MirrorStatusToMessageTypeMapping[status]
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
	slog.Info("send message", slog.Any("response", resp))
	return nil
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

func (w *RepoSyncWorker) checkSourceURL(ctx context.Context, sourceURL string) error {
	// Only check huggingface.co URLs
	if !strings.Contains(sourceURL, types.HUGGINGFACE_HOST) {
		return nil
	}
	checkURL := sourceURL + "/info/refs?service=git-upload-pack"
	checkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create check request: %w", err)
	}
	checkResp, err := w.httpClient.Do(checkReq)
	if err != nil {
		return fmt.Errorf("failed to check source url %s: %w", checkURL, err)
	}
	defer checkResp.Body.Close()

	if checkResp.StatusCode != http.StatusOK {
		return fmt.Errorf("source url %s check failed with status code: %d", checkURL, checkResp.StatusCode)
	}

	return nil
}
