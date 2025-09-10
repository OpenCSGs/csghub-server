//go:build !saas

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	msgMq "opencsg.com/csghub-server/mq"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/cache"
)

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
