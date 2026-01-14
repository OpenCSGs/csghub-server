//go:build !saas

package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	msgMq "opencsg.com/csghub-server/mq"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/cache"
)

type extendRepoImpl struct{}
type advancedRepoInterface interface{}
type communityRepoInterface interface {
	MirrorFromSaas(ctx context.Context, namespace, name, currentUser string, repoType types.RepositoryType) error
}

func NewRepoComponent(config *config.Config) (RepoComponent, error) {
	c := &repoComponentImpl{}
	c.namespaceStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.orgStore = database.NewOrgStore()
	c.repoStore = database.NewRepoStore()
	c.repoFileStore = database.NewRepoFileStore()
	c.repoRelationsStore = database.NewRepoRelationsStore()
	c.userLikesStore = database.NewUserLikesStore()
	c.mirrorStore = database.NewMirrorStore()
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.tokenStore = database.NewAccessTokenStore()
	c.syncVersionStore = database.NewSyncVersionStore()
	c.syncClientSettingStore = database.NewSyncClientSettingStore()
	c.fileStore = database.NewFileStore()
	c.mirrorTaskStore = database.NewMirrorTaskStore()
	var err error
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	if config.GitServer.Type == types.GitServerTypeGitea {
		c.mirrorServer, err = git.NewMirrorServer(config)
		if err != nil {
			newError := fmt.Errorf("fail to create git mirror server,error:%w", err)
			slog.Error(newError.Error())
			return nil, newError
		}
	}
	c.tagComponent, err = NewTagComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create tag component,error:%w", err)
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
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.deployTaskStore = database.NewDeployTaskStore()
	c.deployer = deploy.NewDeployer()
	c.publicRootDomain = config.Space.PublicRootDomain
	c.serverBaseUrl = config.APIServer.PublicDomain
	c.clusterInfoStore = database.NewClusterInfoStore()
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.accountingComponent, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.userResourcesStore = database.NewUserResourcesStore()
	c.recomStore = database.NewRecomStore()
	c.config = config
	c.syncCache, err = cache.NewCache(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	syncClientSettingStore := database.NewSyncClientSettingStore()
	setting, err := syncClientSettingStore.First(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get sync client setting: %w", err)
	}
	apiDomain := config.MultiSync.SaasAPIDomain
	c.multiSyncClient = multisync.FromOpenCSG(apiDomain, setting.Token)
	c.sysMQ = msgMq.SystemMQ
	c.notificationSvcClient = rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.mirrorSvcClient = rpc.NewMirrorSvcClient(fmt.Sprintf("%s:%d", config.LfsSync.Host, config.LfsSync.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return c, nil
}

func (c *repoComponentImpl) CheckAccountAndResource(ctx context.Context, userName string, clusterID string, orderDetailID int64, resource *database.SpaceResource) error {
	var hardware types.HardWare
	err := json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return fmt.Errorf("invalid hardware setting, %w", err)
	}
	_, err = c.deployer.CheckResourceAvailable(ctx, clusterID, 0, &hardware)
	if err != nil {
		return fmt.Errorf("fail to check resource, %w", err)
	}
	return nil
}

func (c *repoComponentImpl) allowPublic(repo *database.Repository) (allow bool, reason string) {
	//always allow public repo in on-premises deployment
	return true, ""
}

func (c *repoComponentImpl) RemoteDiff(ctx context.Context, req types.GetDiffBetweenCommitsReq) ([]types.RemoteDiffs, error) {
	return c.multiSyncClient.Diff(ctx, types.RemoteDiffReq{
		RepoType:     req.RepoType,
		Namespace:    req.Namespace,
		Name:         req.Name,
		LeftCommitID: req.LeftCommitID,
	})
}

func (c *repoComponentImpl) IsXnetEnabled(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (*types.XetEnabled, error) {
	return nil, nil
}

func (c *repoComponentImpl) MirrorProgress(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) (types.LFSSyncProgressResp, error) {
	var progressResp types.LFSSyncProgressResp

	return progressResp, nil
}

func (c *repoComponentImpl) MirrorFromSaas(ctx context.Context, namespace, name, currentUser string, repoType types.RepositoryType) error {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	m, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to find mirror, error: %w", err)
		}
	}
	if m != nil {
		err := c.mirrorFromSaasSync(ctx, m, repo, namespace, name, repoType)
		if err != nil {
			return fmt.Errorf("failed to trigger mirror sync, error: %w", err)
		}
		return nil
	}

	var mirror database.Mirror
	syncVersion, err := c.syncVersionStore.FindByRepoTypeAndPath(ctx, repo.PathWithOutPrefix(), repoType)
	if err != nil {
		return fmt.Errorf("failed to find sync version, error: %w", err)
	}
	mirrorSource := &database.MirrorSource{}
	if syncVersion.SourceID == types.SyncVersionSourceOpenCSG {
		mirrorSource.SourceName = types.OpenCSGPrefix
	} else if syncVersion.SourceID == types.SyncVersionSourceHF {
		mirrorSource.SourceName = types.HuggingfacePrefix
	}

	mirrorSource.SourceName = types.OpenCSGPrefix
	syncClientSetting, err := c.syncClientSettingStore.First(ctx)
	if err != nil {
		return fmt.Errorf("failed to find sync client setting, error: %w", err)
	}

	sourceUrl := common.TrimPrefixCloneURLBySourceID(c.config.MultiSync.SaasSyncDomain, string(repoType), namespace, name, syncVersion.SourceID)
	mirror.SourceUrl = sourceUrl
	mirror.MirrorSourceID = mirrorSource.ID
	mirror.RepositoryID = repo.ID
	mirror.Repository = repo
	mirror.Username = currentUser
	mirror.AccessToken = c.config.Mirror.Token
	mirror.SourceRepoPath = fmt.Sprintf("%s/%s", namespace, name)

	taskId, err := c.git.CreateMirrorRepo(ctx, gitserver.CreateMirrorRepoReq{
		Namespace:   namespace,
		Name:        name,
		CloneUrl:    mirror.SourceUrl,
		RepoType:    repoType,
		MirrorToken: syncClientSetting.Token,
		Private:     false,
	})
	if err != nil {
		return fmt.Errorf("failed to create mirror repo in git: %w", err)
	}

	mirror.MirrorTaskID = taskId

	m, err = c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return fmt.Errorf("failed to create mirror: %w", err)
	}

	mt := database.MirrorTask{
		MirrorID: m.ID,
		Priority: m.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.CancelOtherTasksAndCreate(ctx, mt)
	if err != nil {
		return fmt.Errorf("failed to create mirror task: %w", err)
	}

	repo.SyncStatus = types.SyncStatusPending

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) mirrorFromSaasSync(ctx context.Context, mirror *database.Mirror, repo *database.Repository, namespace, name string, repoType types.RepositoryType) error {
	var err error
	mt := database.MirrorTask{
		MirrorID: mirror.ID,
		Priority: mirror.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.CancelOtherTasksAndCreate(ctx, mt)
	if err != nil {
		return fmt.Errorf("failed to create mirror task: %w", err)
	}

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}

	return nil
}
