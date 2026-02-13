package component

import (
	"github.com/google/wire"
	mock_accounting "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mock_dataviewer_client "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/dataviewer"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_mirror "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/mirrorserver"
	mock_importer "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/importer"
	mock_multisync "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/multisync"
	mock_preader "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/parquet"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_rsa "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rsa"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_checker "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/checker"
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/component/checker"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/importer"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/parquet"

	// "opencsg.com/csghub-server/builder/parquet"
	"opencsg.com/csghub-server/builder/dataviewer"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/rsa"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/mirror/cache"
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
	mock_component.NewMockImportComponent,
	wire.Bind(new(ImportComponent), new(*mock_component.MockImportComponent)),
	mock_component.NewMockSpaceTemplateComponent,
	wire.Bind(new(SpaceTemplateComponent), new(*mock_component.MockSpaceTemplateComponent)),
	mock_component.NewMockMCPServerComponent,
	wire.Bind(new(MCPServerComponent), new(*mock_component.MockMCPServerComponent)),
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

var MockedS3CoreSet = wire.NewSet(
	mock_s3.NewMockCore,
	wire.Bind(new(s3.Core), new(*mock_s3.MockCore)),
)

var MockedMultiSyncClientSet = wire.NewSet(
	mock_multisync.NewMockClient,
	wire.Bind(new(multisync.Client), new(*mock_multisync.MockClient)),
)

var MockedDeployerSet = wire.NewSet(
	mock_deploy.NewMockDeployer,
	wire.Bind(new(deploy.Deployer), new(*mock_deploy.MockDeployer)),
)

var MockedCacheSet = wire.NewSet(
	mock_cache.NewMockCache,
	wire.Bind(new(cache.Cache), new(*mock_cache.MockCache)),
)

var MockedMirrorServerSet = wire.NewSet(
	mock_mirror.NewMockMirrorServer,
	wire.Bind(new(mirrorserver.MirrorServer), new(*mock_mirror.MockMirrorServer)),
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

var MockedXnetSvcClientSet = wire.NewSet(
	mock_rpc.NewMockXnetSvcClient,
	wire.Bind(new(rpc.XnetSvcClient), new(*mock_rpc.MockXnetSvcClient)),
)

var MockedDataviewerClientSet = wire.NewSet(
	mock_dataviewer_client.NewMockDataviewerClient,
	wire.Bind(new(dataviewer.DataviewerClient), new(*mock_dataviewer_client.MockDataviewerClient)),
)

var MockedRsaReader = wire.NewSet(
	mock_rsa.NewMockKeysReader,
	wire.Bind(new(rsa.KeysReader), new(*mock_rsa.MockKeysReader)),
)

var MockedImporterSet = wire.NewSet(
	mock_importer.NewMockImporter,
	wire.Bind(new(importer.Importer), new(*mock_importer.MockImporter)),
)

var MockedCheckerSet = wire.NewSet(
	mock_checker.NewMockGitCallbackChecker,
	wire.Bind(new(checker.GitCallbackChecker), new(*mock_checker.MockGitCallbackChecker)),
)

func ProvideTestConfig() *config.Config {
	return &config.Config{}
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
		deployTaskStore:     stores.DeployTask,
		collectionStore:     stores.Collection,
		accountingComponent: accountingComponent,
		spaceResourceStore:  stores.SpaceResource,
		userResourcesStore:  stores.UserResources,
		promptStore:         stores.Prompt,
		workflowStore:       stores.Workflow,
		mcpServerStore:      stores.MCPServerStore,
	}
}

var UserComponentSet = wire.NewSet(NewTestUserComponent)

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
		runtimeArchitecturesStore: stores.RuntimeArch,
		mirrorStore:               stores.Mirror,
		lfsMetaObjectStore:        stores.LfsMetaObject,
	}
}

var ModelComponentSet = wire.NewSet(NewTestModelComponent)

func NewTestAccountingComponent(stores *tests.MockStores, accountingClient accounting.AccountingClient) *accountingComponentImpl {
	return &accountingComponentImpl{
		accountingClient: accountingClient,
		userStore:        stores.User,
		deployTaskStore:  stores.DeployTask,
	}
}

var AccountingComponentSet = wire.NewSet(NewTestAccountingComponent)

func NewTestGitHTTPComponent(
	config *config.Config,
	stores *tests.MockStores,
	repoComponent RepoComponent,
	gitServer gitserver.GitServer,
	s3Client s3.Client,
	s3Core s3.Core,
	xnetClient rpc.XnetSvcClient,
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
		s3Core:             s3Core,
		mirrorStore:        stores.Mirror,
		xnetClient:         xnetClient,
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

func NewTestMirrorComponent(config *config.Config, stores *tests.MockStores, mirrorServer mirrorserver.MirrorServer, repoComponent RepoComponent, gitServer gitserver.GitServer, s3Client s3.Client) *mirrorComponentImpl {
	return &mirrorComponentImpl{
		tokenStore:                  stores.GitServerAccessToken,
		mirrorServer:                mirrorServer,
		repoComp:                    repoComponent,
		git:                         gitServer,
		s3Client:                    s3Client,
		modelStore:                  stores.Model,
		datasetStore:                stores.Dataset,
		codeStore:                   stores.Code,
		repoStore:                   stores.Repo,
		mirrorStore:                 stores.Mirror,
		mirrorSourceStore:           stores.MirrorSource,
		namespaceStore:              stores.Namespace,
		userStore:                   stores.User,
		config:                      config,
		mirrorTaskStore:             stores.MirrorTaskStore,
		mirrorNamespaceMappingStore: stores.MirrorNamespaceMapping,
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

func NewTestBroadcastComponent(stores *tests.MockStores) *broadcastComponentImpl {
	return &broadcastComponentImpl{
		broadcastStore: stores.Broadcast,
	}
}

var BroadcastComponentSet = wire.NewSet(NewTestBroadcastComponent)

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
		recomStore:         stores.Recom,
		lfsMetaObjectStore: stores.LfsMetaObject,
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
		recomStore:     stores.Recom,
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
		recomStore:       stores.Recom,
		syncVersionStore: stores.SyncVersion,
		tagStore:         stores.Tag,
		fileStore:        stores.File,
		gitServer:        gitServer,
	}
}

var MultiSyncComponentSet = wire.NewSet(NewTestMultiSyncComponent)

func NewTestInternalComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, gitServer gitserver.GitServer, dvClient dataviewer.DataviewerClient, checker checker.GitCallbackChecker) *internalComponentImpl {
	i := &internalComponentImpl{
		config:         config,
		sshKeyStore:    stores.SSH,
		repoStore:      stores.Repo,
		tokenStore:     stores.AccessToken,
		namespaceStore: stores.Namespace,
		repoComponent:  repoComponent,
		gitServer:      gitServer,
		dataviewer:     dvClient,
	}
	i.callbackCheckers = append(i.callbackCheckers, checker)
	return i
}

var InternalComponentSet = wire.NewSet(NewTestInternalComponent)

func NewTestMirrorSourceComponent(config *config.Config, stores *tests.MockStores) *mirrorSourceComponentImpl {
	return &mirrorSourceComponentImpl{
		mirrorSourceStore: stores.MirrorSource,
		userStore:         stores.User,
	}
}

var MirrorSourceComponentSet = wire.NewSet(NewTestMirrorSourceComponent)

var SpaceResourceComponentSet = wire.NewSet(NewTestSpaceResourceComponent)

func NewTestTagComponent(config *config.Config, stores *tests.MockStores, sensitiveChecker rpc.ModerationSvcClient) *tagComponentImpl {
	return &tagComponentImpl{
		tagStore:         stores.Tag,
		repoStore:        stores.Repo,
		sensitiveChecker: sensitiveChecker,
		userStore:        stores.User,
	}
}

var TagComponentSet = wire.NewSet(NewTestTagComponent)

func NewTestRecomComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *recomComponentImpl {
	return &recomComponentImpl{
		recomStore: stores.Recom,
		userStore:  stores.User,
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

func NewTestClusterComponent(config *config.Config, deployer deploy.Deployer, stores *tests.MockStores) *clusterComponentImpl {
	return &clusterComponentImpl{
		deployer:        deployer,
		clusterStore:    stores.ClusterInfo,
		deployTaskStore: stores.DeployTask,
		resStore:        stores.SpaceResource,
		workflowStore:   stores.Workflow,
	}
}

var ClusterComponentSet = wire.NewSet(NewTestClusterComponent)

func NewTestEvaluationComponent(
	config *config.Config,
	stores *tests.MockStores,
	deployer deploy.Deployer,
	accountingComponent AccountingComponent,
	repoComponent RepoComponent,
	userSvcClient rpc.UserSvcClient,
) *evaluationComponentImpl {
	return &evaluationComponentImpl{
		deployer:              deployer,
		userStore:             stores.User,
		modelStore:            stores.Model,
		datasetStore:          stores.Dataset,
		mirrorStore:           stores.Mirror,
		repoStore:             stores.Repo,
		spaceResourceStore:    stores.SpaceResource,
		tokenStore:            stores.AccessToken,
		workflowStore:         stores.Workflow,
		runtimeFrameworkStore: stores.RuntimeFramework,
		config:                config,
		accountingComponent:   accountingComponent,
		repoComponent:         repoComponent,
		userSvcClient:         userSvcClient,
		clusterStore:          stores.ClusterInfo,
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
		userStore:    stores.User,
	}
}

var SyncClientSettingComponentSet = wire.NewSet(NewTestSyncClientSettingComponent)

func NewTestEventComponent(config *config.Config, stores *tests.MockStores) *eventComponentImpl {
	return &eventComponentImpl{
		eventStore: stores.Event,
	}
}

var EventComponentSet = wire.NewSet(NewTestEventComponent)

func NewTestLicenseComponent(config *config.Config, stores *tests.MockStores, keysReader rsa.KeysReader) *licenseComponentImpl {
	return &licenseComponentImpl{
		publicKeyFile:  "publickey.pem",
		privateKeyFile: "privatekey.pem",
		userStore:      stores.User,
		licenseStore:   stores.License,
		keysReader:     keysReader,
	}
}

var LicenseComponentSet = wire.NewSet(NewTestLicenseComponent)

func NewTestImportComponent(config *config.Config, stores *tests.MockStores, repoComponent RepoComponent, importer importer.Importer) *importComponentImpl {
	return &importComponentImpl{
		userStore:         stores.User,
		mirrorStore:       stores.Mirror,
		repoStore:         stores.Repo,
		codeStore:         stores.Code,
		importer:          importer,
		mirrorSourceStore: stores.MirrorSource,
		repoComponent:     repoComponent,
		mirrorTaskStore:   stores.MirrorTaskStore,
	}
}

var ImportComponentSet = wire.NewSet(NewTestImportComponent)

func NewTestSpaceTemplateComponent(config *config.Config, stores *tests.MockStores) *spaceTemplateComponentImpl {
	return &spaceTemplateComponentImpl{
		spaceTemplateStore: stores.SpaceTemplate,
	}
}

var SpaceTemplateComponentSet = wire.NewSet(NewTestSpaceTemplateComponent)

func NewTestRuleComponent(config *config.Config, stores *tests.MockStores) *ruleComponentImpl {
	return &ruleComponentImpl{
		ruleStore: stores.RuleStore,
	}
}

var RuleComponentSet = wire.NewSet(NewTestRuleComponent)

func NewTestMCPServerComponent(config *config.Config, stores *tests.MockStores, rpcUser rpc.UserSvcClient, repoComponent RepoComponent, gitServer gitserver.GitServer) *mcpServerComponentImpl {
	return &mcpServerComponentImpl{
		config:             config,
		repoComponent:      repoComponent,
		repoStore:          stores.Repo,
		gitServer:          gitServer,
		userSvcClient:      rpcUser,
		mcpServerStore:     stores.MCPServerStore,
		userLikesStore:     stores.UserLikes,
		recomStore:         stores.Recom,
		spaceResourceStore: stores.SpaceResource,
		namespaceStore:     stores.Namespace,
	}
}

var MCPServerComponentSet = wire.NewSet(NewTestMCPServerComponent)

func NewTestMCPScannerComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer) *mcpScannerComponentImpl {
	return &mcpScannerComponentImpl{
		llmConfigStore: stores.LLMConfig,
		promptStore:    stores.PromptPrefix,
		gitServer:      gitServer,
	}
}

var MCPScannerComponentSet = wire.NewSet(NewTestMCPScannerComponent)

func NewTestStatComponent(config *config.Config, stores *tests.MockStores, rpcUser rpc.UserSvcClient, repoComponent RepoComponent, gitServer gitserver.GitServer) *statComponentImpl {
	return &statComponentImpl{
		config:          config,
		statSnapStore:   stores.StatSnapStore,
		deployTaskStore: stores.DeployTask,
	}
}

var StatComponentTestSet = wire.NewSet(NewTestStatComponent)

func NewTestLLMServiceComponent(config *config.Config, stores *tests.MockStores) *llmServiceComponentImpl {
	return &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
}

var LLMServiceComponentTestSet = wire.NewSet(NewTestLLMServiceComponent)

func NewTestNotebookComponent(
	stores *tests.MockStores,
	repoComponent RepoComponent,
	deployer deploy.Deployer,
) *notebookComponentImpl {
	return &notebookComponentImpl{
		userStore:              stores.User,
		repoComponent:          repoComponent,
		deployer:               deployer,
		deployTaskStore:        stores.DeployTask,
		spaceResourceStore:     stores.SpaceResource,
		runtimeFrameworksStore: stores.RuntimeFramework,
	}
}

var NotebookComponentSet = wire.NewSet(NewTestNotebookComponent)

func NewTestXnetComponent(
	stores *tests.MockStores,
	xnetClient rpc.XnetSvcClient,
	repoComp RepoComponent,
) *XnetComponentImpl {
	return &XnetComponentImpl{
		repoStore:      stores.Repo,
		xnetClient:     xnetClient,
		userStore:      stores.User,
		namespaceStore: stores.Namespace,
		repoComp:       repoComp,
	}
}

var XnetComponentSet = wire.NewSet(NewTestXnetComponent)
