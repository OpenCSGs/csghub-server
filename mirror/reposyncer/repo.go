package reposyncer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/mirror/hook"
)

var MirrorStatusToMessageTypeMapping = map[types.MirrorTaskStatus]string{
	types.MirrorRepoSyncStart:   "repo_sync_start",
	types.MirrorLfsSyncStart:    "lfs_sync_start",
	types.MirrorLfsSyncFailed:   "sync_failed",
	types.MirrorLfsSyncFinished: "sync_finished",
	types.MirrorRepoTooLarge:    "repo_too_large",
}

type commitCheckpointStore interface {
	UpdateCommitCheckpoint(ctx context.Context, taskID int64, beforeCommitID, afterCommitID string) (database.MirrorTask, error)
}

type RepoSyncWorker struct {
	mirrorTaskStore        database.MirrorTaskStore
	repoStore              database.RepoStore
	promptPrefixStore      database.PromptPrefixStore
	llmConfigStore         database.LLMConfigStore
	syncClientSettingStore database.SyncClientSettingStore
	git                    gitserver.GitServer
	config                 *config.Config
	msgSender              hook.MessageSender
	httpClient             *http.Client
}

func NewRepoSyncWorker(config *config.Config) (*RepoSyncWorker, error) {
	var err error
	w := &RepoSyncWorker{}
	w.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	w.repoStore = database.NewRepoStore()
	w.promptPrefixStore = database.NewPromptPrefixStore(config)
	w.llmConfigStore = database.NewLLMConfigStore(config)
	w.syncClientSettingStore = database.NewSyncClientSettingStore()
	w.mirrorTaskStore = database.NewMirrorTaskStore()
	w.config = config
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

// SyncRepo fetches remote Git data and records commit checkpoints for the following LFS stage.
func (w *RepoSyncWorker) SyncRepo(
	ctx context.Context,
	mirror *database.Mirror,
	mt *database.MirrorTask,
) (*database.MirrorTask, error) {
	// Check if repository is not present
	if mirror.Repository == nil {
		return mt, fmt.Errorf("mirror repository is nil")
	}
	mt.Mirror = mirror

	// Send Message
	err := w.sendMessage(ctx, mirror, types.MirrorRepoSyncStart)
	if err != nil {
		slog.Error("failed to send notice message", slog.Any("error", err))
	}

	namespace, name, err := common.GetNamespaceAndNameFromPath(mirror.Repository.Path)
	if err != nil {
		return mt, fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
	}
	relativePath := mirror.Repository.GitalyPath()

	// Check if the repository already exists, if not, create it
	err = w.ensureRepoExists(
		ctx,
		namespace,
		name,
		mirror.Repository.DefaultBranch,
		mirror.Repository.RepositoryType,
		relativePath,
	)
	if err != nil {
		return mt, fmt.Errorf("failed to ensure repository exists: %w", err)
	}

	// Get before last commit id
	beforeCommitID := mt.BeforeLastCommitID
	if beforeCommitID == "" && mt.AfterLastCommitID == "" {
		commitBefore, err := w.getRepoLastCommit(
			ctx,
			namespace,
			name,
			mirror.Repository.DefaultBranch,
			mirror.Repository.RepositoryType,
			relativePath,
		)
		if err == nil && commitBefore != nil {
			beforeCommitID = commitBefore.ID
			mt.BeforeLastCommitID = beforeCommitID
			if err := w.updateCommitCheckpoint(ctx, mt, beforeCommitID, ""); err != nil {
				return mt, err
			}
		} else if err != nil {
			slog.Info(
				"skip before commit checkpoint because last commit is unavailable",
				slog.Any("error", err),
				slog.Any("repo_type", mirror.Repository.RepositoryType),
				slog.Any("namespace", namespace),
				slog.Any("name", name),
			)
		}
	}

	slog.Info(
		"Start to sync mirror repo",
		slog.Any("repo_type", mirror.Repository.RepositoryType),
		slog.Any("namespace", namespace),
		slog.Any("name", name),
	)

	req := gitserver.MirrorSyncReq{
		Namespace:    namespace,
		Name:         name,
		CloneUrl:     mirror.SourceUrl,
		Username:     mirror.Username,
		AccessToken:  mirror.AccessToken,
		RepoType:     mirror.Repository.RepositoryType,
		RelativePath: relativePath,
	}
	if mirror.Repository.IsOpenCSGRepo() && !w.config.Saas {
		syncClientSetting, err := w.syncClientSettingStore.First(ctx)
		if err != nil {
			return mt, fmt.Errorf("failed to find sync client setting, error: %w", err)
		}
		req.MirrorToken = syncClientSetting.Token
	}

	if err := w.checkSourceURL(ctx, mirror.SourceUrl, mirror.Username, mirror.AccessToken); err != nil {
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
		Namespace:    namespace,
		Name:         name,
		RepoType:     mirror.Repository.RepositoryType,
		RelativePath: relativePath,
	})
	if err != nil {
		return mt, fmt.Errorf("failed to get repo, error: %w", err)
	}

	parts := strings.Split(resp.DefaultBranch, "/")
	branch := parts[len(parts)-1]
	mirror.Repository.DefaultBranch = branch

	slog.Info(
		"Resolved mirror repo default branch",
		slog.Any("repo_type", mirror.Repository.RepositoryType),
		slog.Any("namespace", namespace),
		slog.Any("name", name),
	)

	// Get repo last commit
	commit, err := w.getRepoLastCommit(
		ctx,
		namespace,
		name,
		branch,
		mirror.Repository.RepositoryType,
		relativePath,
	)
	if err != nil {
		return mt, fmt.Errorf("failed to get repo last commit: %w", err)
	}

	mt.BeforeLastCommitID = beforeCommitID
	mt.AfterLastCommitID = commit.ID
	if err := w.updateCommitCheckpoint(ctx, mt, "", commit.ID); err != nil {
		return mt, err
	}

	w.generateDescriptionFromReadme(ctx, mirror.Repository.RepositoryType, namespace, name, commit.ID)

	if beforeCommitID != "" && commit.ID != beforeCommitID {
		// Point HEAD to old commit
		slog.Info(
			"Point HEAD to old commit",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)

		err = w.git.UpdateRef(ctx, gitserver.UpdateRefReq{
			Namespace:    namespace,
			Name:         name,
			Ref:          fmt.Sprintf("refs/heads/%s", branch),
			RepoType:     mirror.Repository.RepositoryType,
			NewObjectId:  beforeCommitID,
			RelativePath: relativePath,
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

	if beforeCommitID != "" && commit.ID == beforeCommitID {
		slog.Info(
			"sync repo successfully, no changes detected.",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)
		return mt, nil
	}

	return mt, nil
}

func (w *RepoSyncWorker) updateCommitCheckpoint(ctx context.Context, mt *database.MirrorTask, beforeCommitID, afterCommitID string) error {
	if mt.ID == 0 || w.mirrorTaskStore == nil {
		return nil
	}
	store, ok := w.mirrorTaskStore.(commitCheckpointStore)
	if !ok {
		return nil
	}
	task, err := store.UpdateCommitCheckpoint(ctx, mt.ID, beforeCommitID, afterCommitID)
	if err != nil {
		return fmt.Errorf("failed to update repo sync commit checkpoint: %w", err)
	}
	mt.BeforeLastCommitID = task.BeforeLastCommitID
	mt.AfterLastCommitID = task.AfterLastCommitID
	return nil
}

func (w *RepoSyncWorker) generateDescriptionFromReadme(
	ctx context.Context,
	repoType types.RepositoryType,
	namespace, name, ref string,
) {
	err := component.UpdateRepoDescriptionFromReadme(ctx, component.UpdateRepoDescriptionFromReadmeReq{
		RepoStore:         w.repoStore,
		GitServer:         w.git,
		PromptPrefixStore: w.promptPrefixStore,
		LLMConfigStore:    w.llmConfigStore,
		RepoType:          repoType,
		Namespace:         namespace,
		Name:              name,
		Ref:               ref,
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to generate repository description from readme",
			slog.Any("error", err),
			slog.Any("repo_type", repoType),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("ref", ref),
		)
	}
}

// ensureRepoExists creates missing Git storage at the path loaded with the mirror repository.
func (w *RepoSyncWorker) ensureRepoExists(
	ctx context.Context, namespace, name, branch string,
	repoType types.RepositoryType,
	relativePath string,
) error {
	exists, err := w.git.RepositoryExists(ctx, gitserver.CheckRepoReq{
		Namespace:    namespace,
		Name:         name,
		RepoType:     repoType,
		RelativePath: relativePath,
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
			RelativePath:  relativePath,
		})
		if err != nil {
			return fmt.Errorf("failed to create repo: %w", err)
		}
	}
	return nil
}

// getRepoLastCommit resolves a commit without requiring Gitaly to query repository metadata.
func (w *RepoSyncWorker) getRepoLastCommit(
	ctx context.Context, namespace, name, branch string,
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

// checkSourceURL verifies the Hugging Face Git endpoint with source credentials when provided.
func (w *RepoSyncWorker) checkSourceURL(ctx context.Context, sourceURL, username, accessToken string) error {
	parsedURL, err := url.Parse(sourceURL)
	if err != nil || !strings.EqualFold(parsedURL.Hostname(), types.HUGGINGFACE_HOST) {
		return nil
	}
	checkURL := sourceURL + "/info/refs?service=git-upload-pack"
	checkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create check request: %w", err)
	}
	if username != "" && accessToken != "" {
		checkReq.SetBasicAuth(username, accessToken)
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
