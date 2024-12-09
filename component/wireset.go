package component

import (
	"github.com/google/wire"
	mock_accounting "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_mirror "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/mirrorserver"
	mock_inference "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/inference"
	mock_preader "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/parquet"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_mirror_queue "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/queue"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/inference"
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

var MockedInferenceClientSet = wire.NewSet(
	mock_inference.NewMockClient,
	wire.Bind(new(inference.Client), new(*mock_inference.MockClient)),
)

var MockedAccountingClientSet = wire.NewSet(
	mock_accounting.NewMockAccountingClient,
	wire.Bind(new(accounting.AccountingClient), new(*mock_accounting.MockAccountingClient)),
)

var MockedParquetReaderSet = wire.NewSet(
	mock_preader.NewMockReader,
	wire.Bind(new(parquet.Reader), new(*mock_preader.MockReader)),
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
	inferenceClient  *mock_inference.MockClient
	accountingClient *mock_accounting.MockAccountingClient
	preader          *mock_preader.MockReader
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
	MockedMirrorQueueSet, MockedInferenceClientSet, MockedAccountingClientSet,
	MockedParquetReaderSet,
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
	inferClient inference.Client,
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
		inferClient:               inferClient,
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
