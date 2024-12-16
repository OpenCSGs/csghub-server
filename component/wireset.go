package component

import (
	"github.com/google/wire"
	mock_accounting "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_mirror "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/mirrorserver"
	mock_preader "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/parquet"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_mirror_queue "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/queue"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/parquet"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/mirror/queue"
)

type mockedComponents struct {
	accounting          *mock_component.MockAccountingComponent
	repo                *mock_component.MockRepoComponent
	tag                 *mock_component.MockTagComponent
	space               *mock_component.MockSpaceComponent
	runtimeArchitecture *mock_component.MockRuntimeArchitectureComponent
	sensitive           *mock_component.MockSensitiveComponent
}

var MockedStoreSet = wire.NewSet(
	tests.NewMockStores,
)

var MockedComponentSet = wire.NewSet(
	mock_component.NewMockAccountingComponent,
	wire.Bind(new(AccountingComponent), new(*mock_component.MockAccountingComponent)),
	mock_component.NewMockRepoComponent,
	wire.Bind(new(RepoComponent), new(*mock_component.MockRepoComponent)),
	mock_component.NewMockTagComponent,
	wire.Bind(new(TagComponent), new(*mock_component.MockTagComponent)),
	mock_component.NewMockSpaceComponent,
	wire.Bind(new(SpaceComponent), new(*mock_component.MockSpaceComponent)),
	mock_component.NewMockRuntimeArchitectureComponent,
	wire.Bind(new(RuntimeArchitectureComponent), new(*mock_component.MockRuntimeArchitectureComponent)),
	mock_component.NewMockSensitiveComponent,
	wire.Bind(new(SensitiveComponent), new(*mock_component.MockSensitiveComponent)),
)

var MockedGitServerSet = wire.NewSet(
	mock_git.NewMockGitServer,
	wire.Bind(new(gitserver.GitServer), new(*mock_git.MockGitServer)),
)

var MockedUserSvcSet = wire.NewSet(
	mock_rpc.NewMockUserSvcClient,
	wire.Bind(new(rpc.UserSvcClient), new(*mock_rpc.MockUserSvcClient)),
)

var MockedS3Set = wire.NewSet(
	mock_s3.NewMockClient,
	wire.Bind(new(s3.Client), new(*mock_s3.MockClient)),
)

var MockedDeployerSet = wire.NewSet(
	mock_deploy.NewMockDeployer,
	wire.Bind(new(deploy.Deployer), new(*mock_deploy.MockDeployer)),
)

var MockedMirrorServerSet = wire.NewSet(
	mock_mirror.NewMockMirrorServer,
	wire.Bind(new(mirrorserver.MirrorServer), new(*mock_mirror.MockMirrorServer)),
)

var MockedMirrorQueueSet = wire.NewSet(
	mock_mirror_queue.NewMockPriorityQueue,
	wire.Bind(new(queue.PriorityQueue), new(*mock_mirror_queue.MockPriorityQueue)),
)

var MockedAccountingClientSet = wire.NewSet(
	mock_accounting.NewMockAccountingClient,
	wire.Bind(new(accounting.AccountingClient), new(*mock_accounting.MockAccountingClient)),
)

var MockedParquetReaderSet = wire.NewSet(
	mock_preader.NewMockReader,
	wire.Bind(new(parquet.Reader), new(*mock_preader.MockReader)),
)

var MockedModerationSvcClientSet = wire.NewSet(
	mock_rpc.NewMockModerationSvcClient,
	wire.Bind(new(rpc.ModerationSvcClient), new(*mock_rpc.MockModerationSvcClient)),
)

type Mocks struct {
	stores           *tests.MockStores
	components       *mockedComponents
	gitServer        *mock_git.MockGitServer
	userSvcClient    *mock_rpc.MockUserSvcClient
	s3Client         *mock_s3.MockClient
	mirrorServer     *mock_mirror.MockMirrorServer
	mirrorQueue      *mock_mirror_queue.MockPriorityQueue
	deployer         *mock_deploy.MockDeployer
	accountingClient *mock_accounting.MockAccountingClient
	preader          *mock_preader.MockReader
	moderationClient *mock_rpc.MockModerationSvcClient
}

var AllMockSet = wire.NewSet(
	wire.Struct(new(mockedComponents), "*"),
	wire.Struct(new(Mocks), "*"),
)

func ProvideTestConfig() *config.Config {
	return &config.Config{}
}

var MockSuperSet = wire.NewSet(
	MockedComponentSet, AllMockSet, MockedStoreSet, MockedGitServerSet, MockedUserSvcSet,
	MockedS3Set, MockedDeployerSet, ProvideTestConfig, MockedMirrorServerSet,
	MockedMirrorQueueSet, MockedAccountingClientSet, MockedParquetReaderSet,
	MockedModerationSvcClientSet,
)

func NewTestRepoComponent(config *config.Config, stores *tests.MockStores, rpcUser rpc.UserSvcClient, gitServer gitserver.GitServer, tagComponent TagComponent, s3Client s3.Client, deployer deploy.Deployer, accountingComponent AccountingComponent, mq queue.PriorityQueue, mirrorServer mirrorserver.MirrorServer) *repoComponentImpl {
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
		accountingComponent:    accountingComponent,
		spaceResourceStore:     stores.SpaceResource,
		mq:                     mq,
		mirrorServer:           mirrorServer,
	}
}

var RepoComponentSet = wire.NewSet(NewTestRepoComponent)

func NewTestPromptComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, rpcUser rpc.UserSvcClient, gitServer gitserver.GitServer) *promptComponentImpl {
	config.APIServer.PublicDomain = "https://foo.com"
	config.APIServer.SSHDomain = "ssh://test@127.0.0.1"
	return &promptComponentImpl{
		userStore:         stores.User,
		userLikeStore:     stores.UserLikes,
		promptConvStore:   stores.PromptConversation,
		promptPrefixStore: stores.PromptPrefix,
		llmConfigStore:    stores.LLMConfig,
		promptStore:       stores.Prompt,
		namespaceStore:    stores.Namespace,
		userSvcClient:     rpcUser,
		gitServer:         gitServer,
		repoStore:         stores.Repo,
		llmClient:         llm.NewClient(),
		config:            config,
		repoComponent:     repoComponent,
	}
}

var PromptComponentSet = wire.NewSet(NewTestPromptComponent)

func NewTestUserComponent(
	stores *tests.MockStores,
	gitServer gitserver.GitServer,
	spaceComponent SpaceComponent,
	repoComponent RepoComponent,
	deployer deploy.Deployer,
	accountingComponent AccountingComponent,
) *userComponentImpl {
	return &userComponentImpl{
		userStore:           stores.User,
		modelStore:          stores.Model,
		datasetStore:        stores.Dataset,
		codeStore:           stores.Code,
		spaceStore:          stores.Space,
		namespaceStore:      stores.Namespace,
		gitServer:           gitServer,
		spaceComponent:      spaceComponent,
		repoComponent:       repoComponent,
		deployer:            deployer,
		userLikeStore:       stores.UserLikes,
		repoStore:           stores.Repo,
		collectionStore:     stores.Collection,
		accountingComponent: accountingComponent,
		promptStore:         stores.Prompt,
	}
}

var UserComponentSet = wire.NewSet(NewTestUserComponent)

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

var SpaceComponentSet = wire.NewSet(NewTestSpaceComponent)

func NewTestModelComponent(
	config *config.Config,
	stores *tests.MockStores,
	repoComponent RepoComponent,
	spaceComponent SpaceComponent,
	deployer deploy.Deployer,
	accountingComponent AccountingComponent,
	runtimeArchComponent RuntimeArchitectureComponent,
	gitServer gitserver.GitServer,
	userSvcClient rpc.UserSvcClient,
) *modelComponentImpl {
	config.APIServer.PublicDomain = "https://foo.com"
	config.APIServer.SSHDomain = "ssh://test@127.0.0.1"
	return &modelComponentImpl{
		config:                    config,
		repoComponent:             repoComponent,
		spaceComponent:            spaceComponent,
		modelStore:                stores.Model,
		repoStore:                 stores.Repo,
		spaceResourceStore:        stores.SpaceResource,
		userStore:                 stores.User,
		deployer:                  deployer,
		accountingComponent:       accountingComponent,
		tagStore:                  stores.Tag,
		runtimeArchComponent:      runtimeArchComponent,
		datasetStore:              stores.Dataset,
		recomStore:                stores.Recom,
		gitServer:                 gitServer,
		userLikesStore:            stores.UserLikes,
		repoRuntimeFrameworkStore: stores.RepoRuntimeFramework,
		deployTaskStore:           stores.DeployTask,
		runtimeFrameworksStore:    stores.RuntimeFramework,
		userSvcClient:             userSvcClient,
	}
}

var ModelComponentSet = wire.NewSet(NewTestModelComponent)

func NewTestAccountingComponent(stores *tests.MockStores, accountingClient accounting.AccountingClient) *accountingComponentImpl {
	return &accountingComponentImpl{
		acctountingClient: accountingClient,
		userStore:         stores.User,
		deployTaskStore:   stores.DeployTask,
	}
}

var AccountingComponentSet = wire.NewSet(NewTestAccountingComponent)

func NewTestDatasetViewerComponent(stores *tests.MockStores, cfg *config.Config, repoComponent RepoComponent, gitServer gitserver.GitServer, preader parquet.Reader) *datasetViewerComponentImpl {
	return &datasetViewerComponentImpl{
		cfg:     cfg,
		preader: preader,
	}
}

var DatasetViewerComponentSet = wire.NewSet(NewTestDatasetViewerComponent)

func NewTestGitHTTPComponent(
	config *config.Config,
	stores *tests.MockStores,
	repoComponent RepoComponent,
	gitServer gitserver.GitServer,
	s3Client s3.Client,
) *gitHTTPComponentImpl {
	config.APIServer.PublicDomain = "https://foo.com"
	config.APIServer.SSHDomain = "ssh://test@127.0.0.1"
	return &gitHTTPComponentImpl{
		config:             config,
		repoComponent:      repoComponent,
		repoStore:          stores.Repo,
		userStore:          stores.User,
		gitServer:          gitServer,
		s3Client:           s3Client,
		lfsMetaObjectStore: stores.LfsMetaObject,
		lfsLockStore:       stores.LfsLock,
	}
}

var GitHTTPComponentSet = wire.NewSet(NewTestGitHTTPComponent)

func NewTestDiscussionComponent(
	stores *tests.MockStores,
) *discussionComponentImpl {
	return &discussionComponentImpl{
		repoStore:       stores.Repo,
		userStore:       stores.User,
		discussionStore: stores.Discussion,
	}
}

var DiscussionComponentSet = wire.NewSet(NewTestDiscussionComponent)

func NewTestRuntimeArchitectureComponent(stores *tests.MockStores, repoComponent RepoComponent, gitServer gitserver.GitServer) *runtimeArchitectureComponentImpl {
	return &runtimeArchitectureComponentImpl{
		repoComponent:             repoComponent,
		repoStore:                 stores.Repo,
		repoRuntimeFrameworkStore: stores.RepoRuntimeFramework,
		runtimeFrameworksStore:    stores.RuntimeFramework,
		runtimeArchStore:          stores.RuntimeArch,
		resouceModelStore:         stores.ResourceModel,
		tagStore:                  stores.Tag,
		gitServer:                 gitServer,
	}
}

var RuntimeArchComponentSet = wire.NewSet(NewTestRuntimeArchitectureComponent)

func NewTestMirrorComponent(config *config.Config, stores *tests.MockStores, mirrorServer mirrorserver.MirrorServer, repoComponent RepoComponent, gitServer gitserver.GitServer, s3Client s3.Client, mq queue.PriorityQueue) *mirrorComponentImpl {
	return &mirrorComponentImpl{
		tokenStore:        stores.GitServerAccessToken,
		mirrorServer:      mirrorServer,
		repoComp:          repoComponent,
		git:               gitServer,
		s3Client:          s3Client,
		modelStore:        stores.Model,
		datasetStore:      stores.Dataset,
		codeStore:         stores.Code,
		repoStore:         stores.Repo,
		mirrorStore:       stores.Mirror,
		mirrorSourceStore: stores.MirrorSource,
		namespaceStore:    stores.Namespace,
		userStore:         stores.User,
		config:            config,
		mq:                mq,
	}
}

var MirrorComponentSet = wire.NewSet(NewTestMirrorComponent)

func NewTestCollectionComponent(stores *tests.MockStores, userSvcClient rpc.UserSvcClient, spaceComponent SpaceComponent) *collectionComponentImpl {
	return &collectionComponentImpl{
		collectionStore: stores.Collection,
		orgStore:        stores.Org,
		repoStore:       stores.Repo,
		userStore:       stores.User,
		userLikesStore:  stores.UserLikes,
		userSvcClient:   userSvcClient,
		spaceComponent:  spaceComponent,
	}
}

var CollectionComponentSet = wire.NewSet(NewTestCollectionComponent)

func NewTestDatasetComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, userSvcClient rpc.UserSvcClient, sensitiveComponent SensitiveComponent, gitServer gitserver.GitServer) *datasetComponentImpl {
	return &datasetComponentImpl{
		config:             config,
		repoComponent:      repoComponent,
		tagStore:           stores.Tag,
		datasetStore:       stores.Dataset,
		repoStore:          stores.Repo,
		namespaceStore:     stores.Namespace,
		userStore:          stores.User,
		sensitiveComponent: sensitiveComponent,
		gitServer:          gitServer,
		userLikesStore:     stores.UserLikes,
		userSvcClient:      userSvcClient,
	}
}

var DatasetComponentSet = wire.NewSet(NewTestDatasetComponent)

func NewTestCodeComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, userSvcClient rpc.UserSvcClient, gitServer gitserver.GitServer) *codeComponentImpl {
	return &codeComponentImpl{
		config:         config,
		repoComponent:  repoComponent,
		codeStore:      stores.Code,
		repoStore:      stores.Repo,
		userLikesStore: stores.UserLikes,
		gitServer:      gitServer,
		userSvcClient:  userSvcClient,
	}
}

var CodeComponentSet = wire.NewSet(NewTestCodeComponent)

func NewTestMultiSyncComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *multiSyncComponentImpl {
	return &multiSyncComponentImpl{
		multiSyncStore:   stores.MultiSync,
		repoStore:        stores.Repo,
		modelStore:       stores.Model,
		datasetStore:     stores.Dataset,
		namespaceStore:   stores.Namespace,
		userStore:        stores.User,
		syncVersionStore: stores.SyncVersion,
		tagStore:         stores.Tag,
		fileStore:        stores.File,
		gitServer:        gitServer,
	}
}

var MultiSyncComponentSet = wire.NewSet(NewTestMultiSyncComponent)

func NewTestInternalComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, gitServer gitserver.GitServer) *internalComponentImpl {
	return &internalComponentImpl{
		config:         config,
		sshKeyStore:    stores.SSH,
		repoStore:      stores.Repo,
		tokenStore:     stores.AccessToken,
		namespaceStore: stores.Namespace,
		repoComponent:  repoComponent,
		gitServer:      gitServer,
	}
}

var InternalComponentSet = wire.NewSet(NewTestInternalComponent)

func NewTestMirrorSourceComponent(config *config.Config, stores *tests.MockStores) *mirrorSourceComponentImpl {
	return &mirrorSourceComponentImpl{
		mirrorSourceStore: stores.MirrorSource,
		userStore:         stores.User,
	}
}

var MirrorSourceComponentSet = wire.NewSet(NewTestMirrorSourceComponent)

func NewTestSpaceResourceComponent(config *config.Config, stores *tests.MockStores, deployer deploy.Deployer, accountComponent AccountingComponent) *spaceResourceComponentImpl {
	return &spaceResourceComponentImpl{
		deployer: deployer,
	}
}

var SpaceResourceComponentSet = wire.NewSet(NewTestSpaceResourceComponent)

func NewTestTagComponent(config *config.Config, stores *tests.MockStores, sensitiveChecker rpc.ModerationSvcClient) *tagComponentImpl {
	return &tagComponentImpl{
		tagStore:         stores.Tag,
		repoStore:        stores.Repo,
		sensitiveChecker: sensitiveChecker,
	}
}

var TagComponentSet = wire.NewSet(NewTestTagComponent)

func NewTestRecomComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *recomComponentImpl {
	return &recomComponentImpl{
		recomStore: stores.Recom,
		repoStore:  stores.Repo,
		gitServer:  gitServer,
	}
}

var RecomComponentSet = wire.NewSet(NewTestRecomComponent)

func NewTestSpaceSdkComponent(config *config.Config, stores *tests.MockStores) *spaceSdkComponentImpl {
	return &spaceSdkComponentImpl{
		spaceSdkStore: stores.SpaceSdk,
	}
}

var SpaceSdkComponentSet = wire.NewSet(NewTestSpaceSdkComponent)

func NewTestTelemetryComponent(config *config.Config, stores *tests.MockStores) *telemetryComponentImpl {
	return &telemetryComponentImpl{
		telemetryStore: stores.Telemetry,
		userStore:      stores.User,
		repoStore:      stores.Repo,
	}
}

var TelemetryComponentSet = wire.NewSet(NewTestTelemetryComponent)

func NewTestClusterComponent(config *config.Config, deployer deploy.Deployer) *clusterComponentImpl {
	return &clusterComponentImpl{
		deployer: deployer,
	}
}

var ClusterComponentSet = wire.NewSet(NewTestClusterComponent)

func NewTestEvaluationComponent(config *config.Config, stores *tests.MockStores, deployer deploy.Deployer, accountingComponent AccountingComponent) *evaluationComponentImpl {
	return &evaluationComponentImpl{
		deployer:              deployer,
		userStore:             stores.User,
		modelStore:            stores.Model,
		datasetStore:          stores.Dataset,
		mirrorStore:           stores.Mirror,
		spaceResourceStore:    stores.SpaceResource,
		tokenStore:            stores.AccessToken,
		runtimeFrameworkStore: stores.RuntimeFramework,
		config:                config,
		accountingComponent:   accountingComponent,
	}
}

var EvaluationComponentSet = wire.NewSet(NewTestEvaluationComponent)

func NewTestHFDatasetComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, gitServer gitserver.GitServer) *hFDatasetComponentImpl {
	return &hFDatasetComponentImpl{
		repoComponent: repoComponent,
		tagStore:      stores.Tag,
		datasetStore:  stores.Dataset,
		repoStore:     stores.Repo,
		gitServer:     gitServer,
	}
}

var HFDatasetComponentSet = wire.NewSet(NewTestHFDatasetComponent)

func NewTestRepoFileComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *repoFileComponentImpl {
	return &repoFileComponentImpl{
		repoFileStore: stores.RepoFile,
		repoStore:     stores.Repo,
		gitServer:     gitServer,
	}
}

var RepoFileComponentSet = wire.NewSet(NewTestRepoFileComponent)

func NewTestSensitiveComponent(config *config.Config, checker rpc.ModerationSvcClient) *sensitiveComponentImpl {
	return &sensitiveComponentImpl{
		checker: checker,
	}
}

var SensitiveComponentSet = wire.NewSet(NewTestSensitiveComponent)

func NewTestSSHKeyComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *sSHKeyComponentImpl {
	return &sSHKeyComponentImpl{
		sshKeyStore: stores.SSH,
		userStore:   stores.User,
		gitServer:   gitServer,
	}
}

var SSHKeyComponentSet = wire.NewSet(NewTestSSHKeyComponent)

func NewTestListComponent(config *config.Config, stores *tests.MockStores) *listComponentImpl {
	return &listComponentImpl{
		modelStore:   stores.Model,
		datasetStore: stores.Dataset,
		spaceStore:   stores.Space,
	}
}

var ListComponentSet = wire.NewSet(NewTestListComponent)

func NewTestSyncClientSettingComponent(config *config.Config, stores *tests.MockStores) *syncClientSettingComponentImpl {
	return &syncClientSettingComponentImpl{
		settingStore: stores.SyncClientSetting,
	}
}

var SyncClientSettingComponentSet = wire.NewSet(NewTestSyncClientSettingComponent)

func NewTestEventComponent(config *config.Config, stores *tests.MockStores) *eventComponentImpl {
	return &eventComponentImpl{
		eventStore: stores.Event,
	}
}

var EventComponentSet = wire.NewSet(NewTestEventComponent)
