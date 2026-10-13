package reposyncer

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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

// RepoSyncResult carries the synced mirror task plus signals discovered during
// repo sync. NoLFS reports that the repository has no LFS objects, so the
// caller can finish the task without entering the LFS queue.
type RepoSyncResult struct {
	Task  *database.MirrorTask
	NoLFS bool
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
	// workflowClient starts the git push callback workflow when a sync finishes
	// without LFS objects. It is initialized once the temporal client is up.
	workflowClient temporal.Client
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
	w.workflowClient = temporal.GetClient()
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
// When the synced repository has no LFS objects, it publishes the new commit and triggers the
// git callback immediately, returning NoLFS=true so the caller can finish the task without the
// LFS queue.
func (w *RepoSyncWorker) SyncRepo(
	ctx context.Context,
	mirror *database.Mirror,
	mt *database.MirrorTask,
) (RepoSyncResult, error) {
	// Check if repository is not present
	if mirror.Repository == nil {
		return RepoSyncResult{}, fmt.Errorf("mirror repository is nil")
	}
	mt.Mirror = mirror

	// Send Message
	err := w.sendMessage(ctx, mirror, types.MirrorRepoSyncStart)
	if err != nil {
		slog.Error("failed to send notice message", slog.Any("error", err))
	}

	namespace, name, err := common.GetNamespaceAndNameFromPath(mirror.Repository.Path)
	if err != nil {
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to get namespace and name from mirror repository path: %w", err)
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
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to ensure repository exists: %w", err)
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
				return RepoSyncResult{Task: mt}, err
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
			return RepoSyncResult{Task: mt}, fmt.Errorf("failed to find sync client setting, error: %w", err)
		}
		req.MirrorToken = syncClientSetting.Token
	}

	if err := w.checkSourceURL(ctx, mirror.SourceUrl, mirror.Username, mirror.AccessToken); err != nil {
		return RepoSyncResult{Task: mt}, err
	}

	err = w.git.MirrorSync(ctx, req)
	if err != nil {
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to sync mirror repo, error: %w", err)
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
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to get repo, error: %w", err)
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
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to get repo last commit: %w", err)
	}
	if beforeCommitID != "" && commit.ID != beforeCommitID {
		// Point HEAD to old commit
		slog.Info(
			"Point HEAD to old commit",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)

		err = w.updateRepoRef(ctx, mirror.Repository, namespace, name, branch, beforeCommitID)
		if err != nil {
			return RepoSyncResult{Task: mt}, fmt.Errorf("failed to point HEAD to old commit: %w", err)
		}
		slog.Info(
			"Point HEAD to old commit successfully",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)
	}

	commit, err = w.ensureSkillMetadataFile(ctx, skillMetadataEnsureRequest{
		repository:      mirror.Repository,
		namespace:       namespace,
		name:            name,
		branch:          branch,
		restoreCommitID: beforeCommitID,
		commit:          commit,
	})
	if err != nil {
		return RepoSyncResult{Task: mt}, err
	}

	mt.BeforeLastCommitID = beforeCommitID
	mt.AfterLastCommitID = commit.ID
	if err := w.updateCommitCheckpoint(ctx, mt, "", commit.ID); err != nil {
		return RepoSyncResult{Task: mt}, err
	}

	w.generateDescriptionFromReadme(ctx, mirror.Repository.RepositoryType, namespace, name, commit.ID)

	// Fast path: repos with no LFS objects publish the new commit immediately and
	// skip the LFS queue entirely. Code and skill repos usually have no LFS files,
	// so this avoids waiting for an empty LFS sync phase just to flip HEAD.
	lfsPointers, err := w.git.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace:    namespace,
		Name:         name,
		RepoType:     mirror.Repository.RepositoryType,
		RelativePath: relativePath,
		Limit:        1,
	})
	if err != nil {
		return RepoSyncResult{Task: mt}, fmt.Errorf("failed to check lfs pointers: %w", err)
	}
	if len(lfsPointers) == 0 {
		if err := PublishCommitAndTriggerCallback(
			ctx, w.git, w.workflowClient,
			mirror.Repository, namespace, name, relativePath, commit.ID,
		); err != nil {
			return RepoSyncResult{Task: mt}, fmt.Errorf("failed to publish commit without lfs: %w", err)
		}
		return RepoSyncResult{Task: mt, NoLFS: true}, nil
	}

	if beforeCommitID != "" && commit.ID == beforeCommitID {
		slog.Info(
			"sync repo successfully, no changes detected.",
			slog.Any("repo_type", mirror.Repository.RepositoryType),
			slog.Any("namespace", namespace),
			slog.Any("name", name),
		)
	}

	return RepoSyncResult{Task: mt, NoLFS: false}, nil
}

// ensureSkillMetadataFile adds the required metadata file when an imported
// skill repository does not provide one and returns the resulting HEAD commit.
type skillMetadataEnsureRequest struct {
	repository      *database.Repository
	namespace       string
	name            string
	branch          string
	restoreCommitID string
	commit          *types.Commit
}

func (w *RepoSyncWorker) ensureSkillMetadataFile(ctx context.Context, req skillMetadataEnsureRequest) (result *types.Commit, err error) {
	repository := req.repository
	commit := req.commit
	if repository.RepositoryType != types.SkillRepo {
		return commit, nil
	}
	if _, err := w.git.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: req.namespace,
		Name:      req.name,
		RepoType:  repository.RepositoryType,
		Path:      "SKILL.md",
		Ref:       commit.ID,
	}); err != nil {
		// GetRepoFileRaw maps both gRPC NotFound and InvalidArgument to
		// ErrGitFileNotFound. Only an explicit NotFound means SKILL.md is
		// genuinely absent; InvalidArgument (e.g. an invalid revision) is a
		// real read failure and must not trigger metadata generation on top of
		// a broken state.
		if !errors.Is(err, errorx.ErrGitFileNotFound) || status.Code(err) != codes.NotFound {
			return commit, fmt.Errorf("failed to check imported skill SKILL.md: %w", err)
		}
	} else {
		return commit, nil
	}

	username := repository.User.Username
	if username == "" {
		username = "opencsg"
	}
	email := repository.User.Email
	if email == "" {
		email = fmt.Sprintf("%s@users.noreply.csghub.com", username)
	}
	skillName := repository.Name
	if skillName == "" {
		skillName = req.name
	}
	content, err := buildDefaultSkillMetadata(skillName, repository.Description)
	if err != nil {
		return commit, err
	}
	// CommitFiles may move the branch even when it reports an error. Restore
	// every exit after this point, including when the imported commit already
	// equals the previous commit and no initial switch is necessary.
	defer func() {
		err = w.restoreSkillRef(req, err)
	}()
	if req.restoreCommitID != "" && req.restoreCommitID != commit.ID {
		if err := w.updateRepoRef(ctx, repository, req.namespace, req.name, req.branch, commit.ID); err != nil {
			return commit, fmt.Errorf("failed to switch to imported skill commit before creating SKILL.md: %w", err)
		}
	}
	if err := w.git.CommitFiles(ctx, gitserver.CommitFilesReq{
		Namespace: req.namespace,
		Name:      req.name,
		RepoType:  repository.RepositoryType,
		Revision:  req.branch,
		Username:  username,
		Email:     email,
		Message:   "Add default SKILL.md",
		Files: []gitserver.CommitFile{{
			Path: "SKILL.md", Content: base64.StdEncoding.EncodeToString([]byte(content)), Action: gitserver.CommitActionCreate,
		}},
	}); err != nil {
		return commit, fmt.Errorf("failed to create default SKILL.md: %w", err)
	}
	generatedCommit, err := w.getRepoLastCommit(
		ctx, req.namespace, req.name, req.branch, repository.RepositoryType, repository.GitalyPath(),
	)
	if err != nil {
		return commit, fmt.Errorf("failed to get commit after creating default SKILL.md: %w", err)
	}
	return generatedCommit, nil
}

func buildDefaultSkillMetadata(name, description string) (string, error) {
	metadata, err := yaml.Marshal(struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal default SKILL.md metadata: %w", err)
	}
	return fmt.Sprintf("---\n%s---", metadata), nil
}

// skillRefRestoreTimeout bounds branch restoration after metadata generation.
// Restoration runs independently of task cancellation on both success and failure.
const skillRefRestoreTimeout = 30 * time.Second

func (w *RepoSyncWorker) restoreSkillRef(
	req skillMetadataEnsureRequest,
	cause error,
) error {
	if req.restoreCommitID == "" {
		return cause
	}
	ctx, cancel := context.WithTimeout(context.Background(), skillRefRestoreTimeout)
	defer cancel()
	if err := w.updateRepoRef(
		ctx, req.repository, req.namespace, req.name, req.branch, req.restoreCommitID,
	); err != nil {
		return errors.Join(cause, fmt.Errorf("failed to restore previous commit after SKILL.md generation: %w", err))
	}
	return cause
}

func (w *RepoSyncWorker) updateRepoRef(
	ctx context.Context,
	repository *database.Repository,
	namespace, name, branch, commitID string,
) error {
	return w.git.UpdateRef(ctx, gitserver.UpdateRefReq{
		Namespace:    namespace,
		Name:         name,
		Ref:          fmt.Sprintf("refs/heads/%s", branch),
		RepoType:     repository.RepositoryType,
		NewObjectId:  commitID,
		RelativePath: repository.GitalyPath(),
	})
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
