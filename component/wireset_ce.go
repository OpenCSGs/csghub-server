//go:build !saas && !ee

package component

import (
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/mirror/cache"
)

func NewTestSpaceComponent(
	stores *tests.MockStores,
	repoComponent RepoComponent,
	git gitserver.GitServer,
	deployer deploy.Deployer,
	accountingComponent AccountingComponent,
	config *config.Config,
	userSvcClient rpc.UserSvcClient,
) *spaceComponentImpl {
	return &spaceComponentImpl{
		repoComponent:       repoComponent,
		git:                 git,
		spaceStore:          stores.Space,
		spaceSdkStore:       stores.SpaceSdk,
		spaceResourceStore:  stores.SpaceResource,
		repoStore:           stores.Repo,
		userStore:           stores.User,
		deployer:            deployer,
		publicRootDomain:    config.Space.PublicRootDomain,
		accountingComponent: accountingComponent,
		serverBaseUrl:       config.APIServer.PublicDomain,
		userLikesStore:      stores.UserLikes,
		config:              config,
		userSvcClient:       userSvcClient,
		deployTaskStore:     stores.DeployTask,
	}
}

func NewTestSpaceResourceComponent(config *config.Config, stores *tests.MockStores, deployer deploy.Deployer, accountComponent AccountingComponent) *spaceResourceComponentImpl {
	return &spaceResourceComponentImpl{
		spaceResourceStore: stores.SpaceResource,
		deployer:           deployer,
		userStore:          stores.User,
		accountComponent:   accountComponent,
	}
}

func NewTestMirrorNamespaceMappingComponent(config *config.Config, stores *tests.MockStores) *mirrorNamespaceMappingComponentImpl {
	return &mirrorNamespaceMappingComponentImpl{
		mirrorNamespaceMappingStore: stores.MirrorNamespaceMapping,
	}
}

func NewTestRepoComponent(config *config.Config, stores *tests.MockStores, rpcUser rpc.UserSvcClient, gitServer gitserver.GitServer, tagComponent TagComponent, s3Client s3.Client, deployer deploy.Deployer, cache cache.Cache, accountingComponent AccountingComponent, mirrorServer mirrorserver.MirrorServer, multiSyncClient multisync.Client, xnetClient rpc.XnetSvcClient) *repoComponentImpl {
	return &repoComponentImpl{
		userStore:              stores.User,
		repoStore:              stores.Repo,
		repoRelationsStore:     stores.RepoRelation,
		namespaceStore:         stores.Namespace,
		userSvcClient:          rpcUser,
		config:                 config,
		git:                    gitServer,
		tagComponent:           tagComponent,
		s3Client:               s3Client,
		lfsMetaObjectStore:     stores.LfsMetaObject,
		mirrorStore:            stores.Mirror,
		mirrorSourceStore:      stores.MirrorSource,
		tokenStore:             stores.AccessToken,
		syncVersionStore:       stores.SyncVersion,
		syncClientSettingStore: stores.SyncClientSetting,
		runtimeFrameworksStore: stores.RuntimeFramework,
		deployTaskStore:        stores.DeployTask,
		deployer:               deployer,
		userResourcesStore:     stores.UserResources,
		clusterInfoStore:       stores.ClusterInfo,
		accountingComponent:    accountingComponent,
		spaceResourceStore:     stores.SpaceResource,
		mirrorServer:           mirrorServer,
		fileStore:              stores.File,
		multiSyncClient:        multiSyncClient,
		mirrorTaskStore:        stores.MirrorTaskStore,
		recomStore:             stores.Recom,
		xnetClient:             xnetClient,
	}
}
