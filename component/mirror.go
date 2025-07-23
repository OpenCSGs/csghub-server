package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/cache"
)

type mirrorComponentImpl struct {
	tokenStore         database.GitServerAccessTokenStore
	mirrorServer       mirrorserver.MirrorServer
	saas               bool
	repoComp           RepoComponent
	git                gitserver.GitServer
	s3Client           s3.Client
	lfsBucket          string
	modelStore         database.ModelStore
	datasetStore       database.DatasetStore
	codeStore          database.CodeStore
	repoStore          database.RepoStore
	mirrorStore        database.MirrorStore
	mirrorSourceStore  database.MirrorSourceStore
	namespaceStore     database.NamespaceStore
	lfsMetaObjectStore database.LfsMetaObjectStore
	userStore          database.UserStore
	config             *config.Config
	syncCache          cache.Cache
	mirrorTaskStore    database.MirrorTaskStore
}

type MirrorComponent interface {
	// CreateMirrorRepo often called by the crawler server to create new repo which will then be mirrored from other sources
	CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error)
	Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error)
	Index(ctx context.Context, per, page int, search string) ([]types.Mirror, int, error)
	Statistics(ctx context.Context) ([]types.MirrorStatusCount, error)
}

func NewMirrorComponent(config *config.Config) (MirrorComponent, error) {
	var err error
	c := &mirrorComponentImpl{}
	if config.GitServer.Type == types.GitServerTypeGitea {
		c.mirrorServer, err = git.NewMirrorServer(config)
		if err != nil {
			newError := fmt.Errorf("fail to create git mirror server,error:%w", err)
			slog.Error(newError.Error())
			return nil, newError
		}
	}

	c.repoComp, err = NewRepoComponentImpl(config)
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
	c.mirrorTaskStore = database.NewMirrorTaskStore()
	c.saas = config.Saas
	c.config = config
	syncCache, err := cache.NewCache(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	c.syncCache = syncCache
	return c, nil
}

func (c *mirrorComponentImpl) CreatePushMirrorForFinishedMirrorTask(ctx context.Context) error {
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
func (c *mirrorComponentImpl) CreateMirrorRepo(ctx context.Context, req types.CreateMirrorRepoReq) (*database.Mirror, error) {
	var username string
	namespace := c.mapNamespaceAndName(req.SourceNamespace)
	name := req.SourceName
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check repo existence, error: %w", err)
	}
	if repo != nil {
		name = fmt.Sprintf("%s_%s", req.SourceNamespace, req.SourceName)
		repo, err = c.repoStore.FindByPath(ctx, req.RepoType, namespace, name)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to check repo existence, error: %w", err)
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
	mirror.RepositoryID = repo.ID
	mirror.Repository = repo
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.SourceNamespace, req.SourceName)
	mirror.SourceRepoPath = fmt.Sprintf("%s/%s", req.SourceNamespace, req.SourceName)
	mirror.Priority = types.HighMirrorPriority

	sourceType, sourcePath, err := common.GetSourceTypeAndPathFromURL(req.SourceGitCloneUrl)
	if err == nil {
		err = c.repoStore.UpdateSourcePath(ctx, repo.ID, sourcePath, sourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to update source path in repo: %v", err)
		}
	}

	var taskId int64
	mirror.MirrorTaskID = taskId

	reqMirror, err := c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror")
	}

	reqMirror.Status = types.MirrorQueued
	err = c.mirrorStore.Update(ctx, reqMirror)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror status: %v", err)
	}

	mt := database.MirrorTask{
		MirrorID: reqMirror.ID,
		Status:   types.MirrorQueued,
		Priority: mirror.Priority,
	}

	_, err = c.mirrorTaskStore.Create(ctx, mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror task: %v", err)
	}

	return reqMirror, nil

}

func (m *mirrorComponentImpl) mapNamespaceAndName(sourceNamespace string) string {
	var namespace string
	if ns, found := mirrorOrganizationMap[sourceNamespace]; found {
		namespace = ns
	} else {
		//map all organization to AIWizards if not found
		namespace = "AIWizards"
	}

	return namespace
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
	"opencompass":    "opencompass",
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

func (c *mirrorComponentImpl) Repos(ctx context.Context, per, page int) ([]types.MirrorRepo, int, error) {
	var mirrorRepos []types.MirrorRepo
	mirros, total, err := c.mirrorStore.IndexWithPagination(ctx, per, page, "")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror repositories: %v", err)
	}
	for _, m := range mirros {
		if m.Repository != nil && m.CurrentTask != nil {
			RepoSyncStatus := common.MirrorTaskStatusToRepoStatus(m.CurrentTask.Status)
			mirrorRepos = append(mirrorRepos, types.MirrorRepo{
				ID:         m.ID,
				Path:       m.Repository.Path,
				SyncStatus: RepoSyncStatus,
				RepoType:   m.Repository.RepositoryType,
				Progress:   int8(m.CurrentTask.Progress),
			})
		}
	}
	return mirrorRepos, total, nil
}

func (c *mirrorComponentImpl) Index(ctx context.Context, per, page int, search string) ([]types.Mirror, int, error) {
	var mirrorsResp []types.Mirror
	mirrors, total, err := c.mirrorStore.IndexWithPagination(ctx, per, page, search)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mirror mirrors: %v", err)
	}
	for _, mirror := range mirrors {
		if mirror.Repository != nil {
			var status types.MirrorTaskStatus
			if mirror.CurrentTask != nil {
				status = mirror.CurrentTask.Status
			} else {
				status = types.MirrorQueued
			}
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
				Status:          status,
				Progress:        mirror.Progress,
			})
		}
	}
	return mirrorsResp, total, nil
}

func (c *mirrorComponentImpl) Statistics(ctx context.Context) ([]types.MirrorStatusCount, error) {
	var scs []types.MirrorStatusCount
	statusCounts, err := c.mirrorStore.StatusCount(ctx)
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
