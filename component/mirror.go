package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/queue"
)

type MirrorComponent struct {
	tokenStore         *database.GitServerAccessTokenStore
	mirrorServer       mirrorserver.MirrorServer
	saas               bool
	repoComp           *RepoComponent
	git                gitserver.GitServer
	s3Client           *s3.Client
	lfsBucket          string
	modelStore         *database.ModelStore
	datasetStore       *database.DatasetStore
	codeStore          *database.CodeStore
	repoStore          *database.RepoStore
	mirrorStore        *database.MirrorStore
	mirrorSourceStore  *database.MirrorSourceStore
	namespaceStore     *database.NamespaceStore
	lfsMetaObjectStore *database.LfsMetaObjectStore
	userStore          *database.UserStore
	config             *config.Config
	mq                 *queue.PriorityQueue
}

func NewMirrorComponent(config *config.Config) (*MirrorComponent, error) {
	var err error
	c := &MirrorComponent{}
	c.mirrorServer, err = git.NewMirrorServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git mirror server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.mq, err = queue.GetPriorityQueueInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get priority queue: %v", err)
	}
	c.repoComp, err = NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component,error:%w", err)
	}
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.lfsBucket = config.S3.Bucket
	c.modelStore = database.NewModelStore()
	c.datasetStore = database.NewDatasetStore()
	c.codeStore = database.NewCodeStore()
	c.repoStore = database.NewRepoStore()
	c.mirrorStore = database.NewMirrorStore()
	c.tokenStore = database.NewGitServerAccessTokenStore()
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.userStore = database.NewUserStore()
	c.saas = config.Saas
	c.config = config
	return c, nil
}

func (c *MirrorComponent) CreatePushMirrorForFinishedMirrorTask(ctx context.Context) error {
	mirrors, err := c.mirrorStore.NoPushMirror(ctx)
	if err != nil {
		return fmt.Errorf("fail to find all mirrors, %w", err)
	}

	for _, mirror := range mirrors {
		task, err := c.mirrorServer.GetMirrorTaskInfo(ctx, mirror.MirrorTaskID)
		if err != nil {
			slog.Error("fail to get mirror task info", slog.Int64("taskId", mirror.MirrorTaskID), slog.String("error", err.Error()))
			return fmt.Errorf("fail to get mirror task info, %w", err)
		}
		if task.Status == mirrorserver.TaskStatusFinished {
			err = c.mirrorServer.CreatePushMirror(ctx, mirrorserver.CreatePushMirrorReq{
				Name:        mirror.LocalRepoPath,
				PushUrl:     mirror.PushUrl,
				Username:    mirror.PushUsername,
				AccessToken: mirror.PushAccessToken,
				Interval:    "8h",
			})

			if err != nil {
				slog.Error("fail to create push mirror", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
				continue
			}
			mirror.PushMirrorCreated = true
			err = c.mirrorStore.Update(ctx, &mirror)
			if err != nil {
				slog.Error("fail to update mirror", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
				continue
			}
			slog.Info("create push mirror successfully", slog.Int64("mirrorId", mirror.ID), slog.String("push_url", mirror.PushUrl))
		}
	}
	return nil
}

// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
func (c *MirrorComponent) CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error) {
	var username string
	namespace := c.mapNamespaceAndName(req.SourceNamespace)
	name := req.SourceName
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		return nil, errors.New("user does not have admin permission")
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check repo existance, error: %w", err)
	}
	if repo != nil {
		name = fmt.Sprintf("%s_%s", req.SourceNamespace, req.SourceName)
		repo, err = c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to check repo existance, error: %w", err)
		}
		if repo != nil {
			return nil, fmt.Errorf("repo already exists,repo type:%s, source namespace: %s, source name: %s, target namespace: %s, target name: %s",
				req.RepoType, req.SourceNamespace, req.SourceName, namespace, name)
		}
	}

	dbNamespace, err := c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("namespace does not exist, namespace: %s", namespace)
	}
	username = dbNamespace.User.Username

	// create repo, create mirror repo
	gitRepo, repo, err := c.repoComp.CreateRepo(ctx, types.CreateRepoReq{
		Username:  username,
		Namespace: namespace,
		Name:      name,
		Nickname:  name,
		//TODO: tranlate description automatically
		Description: req.Description,
		//only mirror public repository
		Private:       true,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create OpenCSG repo, error: %w", err)
	}

	if req.RepoType == types.ModelRepo {
		dbModel := database.Model{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.modelStore.Create(ctx, dbModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create model, error: %w", err)
		}
	} else if req.RepoType == types.DatasetRepo {
		dbDataset := database.Dataset{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.datasetStore.Create(ctx, dbDataset)
		if err != nil {
			return nil, fmt.Errorf("failed to create dataset, error: %w", err)
		}
	} else if req.RepoType == types.CodeRepo {
		dbCode := database.Code{
			Repository:   repo,
			RepositoryID: repo.ID,
		}

		_, err := c.codeStore.Create(ctx, dbCode)
		if err != nil {
			return nil, fmt.Errorf("failed to create code, error: %w", err)
		}
	}

	pushAccessToken, err := c.tokenStore.FindByType(ctx, "git")
	if err != nil {
		return nil, fmt.Errorf("failed to find git access token, error: %w", err)
	}
	if len(pushAccessToken) == 0 {
		return nil, fmt.Errorf("failed to find git access token, error: empty table")
	}

	mirrorSource, err := c.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror Source, error: %w", err)
	}
	var mirror database.Mirror
	// mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceGitCloneUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.PushUrl = gitRepo.HttpCloneURL
	mirror.Username = req.SourceNamespace
	mirror.PushUsername = "root"
	//TODO: get user git access token from db git access token
	mirror.PushAccessToken = pushAccessToken[0].Token
	mirror.RepositoryID = repo.ID
	mirror.Repository = repo
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.SourceNamespace, req.SourceName)
	mirror.SourceRepoPath = fmt.Sprintf("%s/%s", req.SourceNamespace, req.SourceName)
	mirror.Priority = types.HighMirrorPriority
	var taskId int64
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		taskId, err = c.mirrorServer.CreateMirrorRepo(ctx, mirrorserver.CreateMirrorRepoReq{
			Namespace: "root",
			Name:      mirror.LocalRepoPath,
			CloneUrl:  mirror.SourceUrl,
			// Username:    req.SourceNamespace,
			// AccessToken: mirror.AccessToken,
			Private: false,
			SyncLfs: req.SyncLfs,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create push mirror in mirror server: %v", err)
		}
	}

	mirror.MirrorTaskID = taskId

	reqMirror, err := c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror")
	}

	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		c.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID: reqMirror.ID,
			Priority: queue.PriorityMap[reqMirror.Priority],
		})
		reqMirror.Status = types.MirrorWaiting
		err = c.mirrorStore.Update(ctx, reqMirror)
		if err != nil {
			return nil, fmt.Errorf("failed to update mirror status: %v", err)
		}
	}

	return reqMirror, nil
}

func (m *MirrorComponent) mapNamespaceAndName(sourceNamespace string) string {
	namespace := sourceNamespace
	if ns, found := mirrorOrganizationMap[sourceNamespace]; found {
		namespace = ns
	} else {
		//map all organization to AIWizards if not found
		namespace = "AIWizards"
	}

	return namespace
}

func (c *MirrorComponent) CheckMirrorProgress(ctx context.Context) error {
	mirrors, err := c.mirrorStore.Unfinished(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unfinished mirrors: %v", err)
	}
	for _, mirror := range mirrors {
		err := c.checkAndUpdateMirrorStatus(ctx, mirror)
		if err != nil {
			slog.Error("fail to check and update mirror status", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
		}
	}
	return nil
}

var mirrorOrganizationMap = map[string]string{
	"THUDM":          "THUDM",
	"baichuan-inc":   "BaiChuanAI",
	"IDEA-CCNL":      "FengShenBang",
	"internlm":       "ShangHaiAILab",
	"pleisto":        "Pleisto",
	"01-ai":          "01AI",
	"codefuse-ai":    "codefuse-ai",
	"WisdomShell":    "WisdomShell",
	"microsoft":      "microsoft",
	"Skywork":        "Skywork",
	"BAAI":           "BAAI",
	"deepseek-ai":    "deepseek-ai",
	"WizardLMTeam":   "WizardLM",
	"IEITYuan":       "IEITYuan",
	"Qwen":           "Qwen",
	"TencentARC":     "TencentARC",
	"OrionStarAI":    "OrionStarAI",
	"openbmb":        "OpenBMB",
	"netease-youdao": "Netease-youdao",
	"ByteDance":      "ByteDance",
}

var mirrorStatusAndRepoSyncStatusMapping = map[types.MirrorTaskStatus]types.RepositorySyncStatus{
	types.MirrorWaiting:    types.SyncStatusPending,
	types.MirrorRunning:    types.SyncStatusInProgress,
	types.MirrorFinished:   types.SyncStatusCompleted,
	types.MirrorFailed:     types.SyncStatusFailed,
	types.MirrorIncomplete: types.SyncStatusFailed,
}

func (c *MirrorComponent) checkAndUpdateMirrorStatus(ctx context.Context, mirror database.Mirror) error {
	var statusAndProgressFunc func(ctx context.Context, mirror database.Mirror) (types.MirrorResp, error)
	if mirror.Repository == nil {
		return nil
	}
	if c.saas {
		statusAndProgressFunc = c.getMirrorStatusAndProgressSaas
	} else {
		statusAndProgressFunc = c.getMirrorStatusAndProgressOnPremise
	}
	mirrorResp, err := statusAndProgressFunc(ctx, mirror)
	if err != nil {
		slog.Error("fail to get mirror status and progress", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
	}
	mirror.Status = mirrorResp.TaskStatus
	mirror.Progress = mirrorResp.Progress
	mirror.LastMessage = mirrorResp.LastMessage
	err = c.mirrorStore.Update(ctx, &mirror)
	if err != nil {
		slog.Error("fail to update mirror", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
		return err
	}
	if mirror.Repository.HTTPCloneURL == "" {
		namespace, name := mirror.Repository.NamespaceAndName()
		repoRes, err := c.git.GetRepo(ctx, gitserver.GetRepoReq{
			Namespace: namespace,
			Name:      name,
			RepoType:  mirror.Repository.RepositoryType,
		})
		if err != nil {
			slog.Error("fail to get repo detail from git server")
		} else {
			mirror.Repository.HTTPCloneURL = common.PortalCloneUrl(repoRes.HttpCloneURL, mirror.Repository.RepositoryType, c.config.GitServer.URL, c.config.Frontend.URL)
			mirror.Repository.SSHCloneURL = repoRes.SshCloneURL
			mirror.Repository.DefaultBranch = repoRes.DefaultBranch
		}
	}
	syncStatus := mirrorStatusAndRepoSyncStatusMapping[mirrorResp.TaskStatus]
	mirror.Repository.SyncStatus = syncStatus
	_, err = c.repoStore.UpdateRepo(ctx, *mirror.Repository)
	if err != nil {
		slog.Error("fail to update repo sync status", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
		return err
	}

	return nil
}

func getAllFiles(namespace, repoName, folder string, repoType types.RepositoryType, ref string, gsTree func(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error)) ([]*types.File, error) {
	var files []*types.File

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      repoName,
		Ref:       ref,
		Path:      folder,
		RepoType:  repoType,
	}
	gitFiles, err := gsTree(context.Background(), getRepoFileTree)
	if err != nil {
		return files, fmt.Errorf("failed to get repo file tree,%w", err)
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			subFiles, err := getAllFiles(namespace, repoName, file.Path, repoType, ref, gsTree)
			if err != nil {
				return files, err
			}
			files = append(files, subFiles...)
		} else {
			files = append(files, file)
		}
	}
	return files, nil
}

func (c *MirrorComponent) getMirrorStatusAndProgressOnPremise(ctx context.Context, mirror database.Mirror) (types.MirrorResp, error) {
	task, err := c.git.GetMirrorTaskInfo(ctx, mirror.MirrorTaskID)
	if err != nil {
		slog.Error("fail to get mirror task info", slog.Int64("taskId", mirror.MirrorTaskID), slog.String("error", err.Error()))
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: "",
			Progress:    0,
		}, fmt.Errorf("fail to get mirror task info, %w", err)
	}
	if task.Status == gitserver.TaskStatusQueued {
		return types.MirrorResp{
			TaskStatus:  types.MirrorWaiting,
			LastMessage: task.Message,
			Progress:    0,
		}, nil
	} else if task.Status == gitserver.TaskStatusRunning {
		progress, err := c.countMirrorProgress(ctx, mirror)
		if err != nil {
			return types.MirrorResp{
				TaskStatus:  types.MirrorRunning,
				LastMessage: task.Message,
				Progress:    0,
			}, err
		}
		return types.MirrorResp{
			TaskStatus:  types.MirrorRunning,
			LastMessage: task.Message,
			Progress:    progress,
		}, nil
	} else if task.Status == gitserver.TaskStatusFailed {
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: task.Message,
			Progress:    0,
		}, nil
	} else if task.Status == gitserver.TaskStatusFinished {
		progress, err := c.countMirrorProgress(ctx, mirror)
		if err != nil {
			return types.MirrorResp{
				TaskStatus:  types.MirrorFailed,
				LastMessage: task.Message,
				Progress:    0,
			}, err
		}
		if progress == 100 {
			return types.MirrorResp{
				TaskStatus:  types.MirrorFinished,
				LastMessage: task.Message,
				Progress:    progress,
			}, nil
		} else {
			return types.MirrorResp{
				TaskStatus:  types.MirrorIncomplete,
				LastMessage: task.Message,
				Progress:    progress,
			}, nil
		}
	} else {
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: "",
			Progress:    0,
		}, nil
	}
}

func (c *MirrorComponent) getMirrorStatusAndProgressSaas(ctx context.Context, mirror database.Mirror) (types.MirrorResp, error) {
	task, err := c.mirrorServer.GetMirrorTaskInfo(ctx, mirror.MirrorTaskID)
	if err != nil {
		slog.Error("fail to get mirror task info", slog.Int64("taskId", mirror.MirrorTaskID), slog.String("error", err.Error()))
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: "",
			Progress:    0,
		}, fmt.Errorf("fail to get mirror task info, %w", err)
	}
	if task.Status == mirrorserver.TaskStatusQueued {
		return types.MirrorResp{
			TaskStatus:  types.MirrorWaiting,
			LastMessage: task.Message,
			Progress:    0,
		}, nil
	} else if task.Status == mirrorserver.TaskStatusRunning {
		progress, err := c.countMirrorProgress(ctx, mirror)
		if err != nil {
			return types.MirrorResp{
				TaskStatus:  types.MirrorRunning,
				LastMessage: task.Message,
				Progress:    0,
			}, err
		}
		return types.MirrorResp{
			TaskStatus:  types.MirrorRunning,
			LastMessage: task.Message,
			Progress:    progress,
		}, nil
	} else if task.Status == mirrorserver.TaskStatusFailed {
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: task.Message,
			Progress:    0,
		}, nil
	} else if task.Status == mirrorserver.TaskStatusFinished {
		progress, err := c.countMirrorProgress(ctx, mirror)
		if err != nil {
			return types.MirrorResp{
				TaskStatus:  types.MirrorFailed,
				LastMessage: task.Message,
				Progress:    0,
			}, err
		}
		if progress == 100 {
			return types.MirrorResp{
				TaskStatus:  types.MirrorFinished,
				LastMessage: task.Message,
				Progress:    progress,
			}, nil
		} else {
			return types.MirrorResp{
				TaskStatus:  types.MirrorIncomplete,
				LastMessage: task.Message,
				Progress:    progress,
			}, nil
		}
	} else {
		return types.MirrorResp{
			TaskStatus:  types.MirrorFailed,
			LastMessage: "",
			Progress:    0,
		}, nil
	}
}

func (c *MirrorComponent) countMirrorProgress(ctx context.Context, mirror database.Mirror) (int8, error) {
	var (
		lfsFiles          []*types.File
		finishedFileCount int
	)
	namespaceAndName := strings.Split(mirror.Repository.Path, "/")
	namespace := namespaceAndName[0]
	name := namespaceAndName[1]
	allFiles, err := getAllFiles(namespace, name, "", mirror.Repository.RepositoryType, "", c.git.GetRepoFileTree)

	if err != nil {
		slog.Error("fail to get all files of mirror repository", slog.Int64("mirrorId", mirror.ID), slog.String("namespace", namespace), slog.String("name", name), slog.String("error", err.Error()))
		return 0, err
	}
	if len(allFiles) == 0 {
		return 0, nil
	}
	for _, f := range allFiles {
		if f.Lfs {
			lfsFiles = append(lfsFiles, f)
		}
	}
	if len(lfsFiles) == 0 {
		return 100, nil
	}
	for _, f := range lfsFiles {
		objectKey := f.LfsRelativePath
		objectKey = path.Join("lfs", objectKey)
		_, err := c.s3Client.StatObject(ctx, c.lfsBucket, objectKey, minio.GetObjectOptions{})
		if err != nil {
			if minio.ToErrorResponse(err).Code != "NoSuchKey" {
				slog.Error("fail to check lfs file", slog.Int64("mirrorId", mirror.ID), slog.String("namespace", namespace), slog.String("name", name), slog.String("filename", f.Path), slog.String("error", err.Error()))
				return 0, err
			}
		} else {
			finishedFileCount += 1
		}
	}

	progress := (finishedFileCount * 100) / len(lfsFiles)
	return int8(progress), nil
}

func (c *MirrorComponent) Repos(ctx context.Context, currentUser string, per, page int) ([]types.MirrorRepo, int, error) {
	var mirrorRepos []types.MirrorRepo
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, 0, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		return nil, 0, errors.New("user does not have admin permission")
	}
	repos, total, err := c.repoStore.WithMirror(ctx, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror repositories: %v", err)
	}
	for _, repo := range repos {
		mirrorRepos = append(mirrorRepos, types.MirrorRepo{
			Path:       repo.Path,
			SyncStatus: repo.SyncStatus,
			RepoType:   repo.RepositoryType,
			Progress:   repo.Mirror.Progress,
		})
	}
	return mirrorRepos, total, nil
}

func (c *MirrorComponent) Index(ctx context.Context, currentUser string, per, page int) ([]types.Mirror, int, error) {
	var mirrorsResp []types.Mirror
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, 0, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		return nil, 0, errors.New("user does not have admin permission")
	}
	mirrors, total, err := c.mirrorStore.IndexWithPagination(ctx, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror mirrors: %v", err)
	}
	for _, mirror := range mirrors {
		mirrorsResp = append(mirrorsResp, types.Mirror{
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
			LastMessage:     mirror.LastMessage,
			Status:          mirror.Status,
			Progress:        mirror.Progress,
		})
	}
	return mirrorsResp, total, nil
}
