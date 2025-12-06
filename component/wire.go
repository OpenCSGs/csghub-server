//go:build wireinject
// +build wireinject

package component

import (
	"context"

	"github.com/google/wire"
	"github.com/stretchr/testify/mock"
)

type testRepoWithMocks struct {
	*repoComponentImpl
	mocks *Mocks
}

func initializeTestRepoComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRepoWithMocks {
	wire.Build(
		MockSuperSet, RepoComponentSet,
		wire.Struct(new(testRepoWithMocks), "*"),
	)
	return &testRepoWithMocks{}
}

type testPromptWithMocks struct {
	*promptComponentImpl
	mocks *Mocks
}

func initializeTestPromptComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testPromptWithMocks {
	wire.Build(
		MockSuperSet, PromptComponentSet,
		wire.Struct(new(testPromptWithMocks), "*"),
	)
	return &testPromptWithMocks{}
}

type testUserWithMocks struct {
	*userComponentImpl
	mocks *Mocks
}

func initializeTestUserComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testUserWithMocks {
	wire.Build(
		MockSuperSet, UserComponentSet,
		wire.Struct(new(testUserWithMocks), "*"),
	)
	return &testUserWithMocks{}
}

type testSpaceWithMocks struct {
	*spaceComponentImpl
	mocks *Mocks
}

func initializeTestSpaceComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSpaceWithMocks {
	wire.Build(
		MockSuperSet, SpaceComponentSet,
		wire.Struct(new(testSpaceWithMocks), "*"),
	)
	return &testSpaceWithMocks{}
}

type testModelWithMocks struct {
	*modelComponentImpl
	mocks *Mocks
}

func initializeTestModelComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testModelWithMocks {
	wire.Build(
		MockSuperSet, ModelComponentSet,
		wire.Struct(new(testModelWithMocks), "*"),
	)
	return &testModelWithMocks{}
}

type testAccountingWithMocks struct {
	*accountingComponentImpl
	mocks *Mocks
}

func initializeTestAccountingComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testAccountingWithMocks {
	wire.Build(
		MockSuperSet, AccountingComponentSet,
		wire.Struct(new(testAccountingWithMocks), "*"),
	)
	return &testAccountingWithMocks{}
}

type testGitHTTPWithMocks struct {
	*gitHTTPComponentImpl
	mocks *Mocks
}

func initializeTestGitHTTPComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testGitHTTPWithMocks {
	wire.Build(
		MockSuperSet, GitHTTPComponentSet,
		wire.Struct(new(testGitHTTPWithMocks), "*"),
	)
	return &testGitHTTPWithMocks{}
}

type testDiscussionWithMocks struct {
	*discussionComponentImpl
	mocks *Mocks
}

func initializeTestDiscussionComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDiscussionWithMocks {
	wire.Build(
		MockSuperSet, DiscussionComponentSet,
		wire.Struct(new(testDiscussionWithMocks), "*"),
	)
	return &testDiscussionWithMocks{}
}

type testRuntimeArchWithMocks struct {
	*runtimeArchitectureComponentImpl
	mocks *Mocks
}

func initializeTestRuntimeArchComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRuntimeArchWithMocks {
	wire.Build(
		MockSuperSet, RuntimeArchComponentSet,
		wire.Struct(new(testRuntimeArchWithMocks), "*"),
	)
	return &testRuntimeArchWithMocks{}
}

type testMirrorWithMocks struct {
	*mirrorComponentImpl
	mocks *Mocks
}

func initializeTestMirrorComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMirrorWithMocks {
	wire.Build(
		MockSuperSet, MirrorComponentSet,
		wire.Struct(new(testMirrorWithMocks), "*"),
	)
	return &testMirrorWithMocks{}
}

type testCollectionWithMocks struct {
	*collectionComponentImpl
	mocks *Mocks
}

func initializeTestCollectionComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testCollectionWithMocks {
	wire.Build(
		MockSuperSet, CollectionComponentSet,
		wire.Struct(new(testCollectionWithMocks), "*"),
	)
	return &testCollectionWithMocks{}
}

type testBroadcastWithMocks struct {
	*broadcastComponentImpl
	mocks *Mocks
}

func initializeTestBroadcastComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testBroadcastWithMocks {
	wire.Build(
		MockSuperSet, BroadcastComponentSet,
		wire.Struct(new(testBroadcastWithMocks), "*"),
	)
	return &testBroadcastWithMocks{}
}

type testDatasetWithMocks struct {
	*datasetComponentImpl
	mocks *Mocks
}

func initializeTestDatasetComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDatasetWithMocks {
	wire.Build(
		MockSuperSet, DatasetComponentSet,
		wire.Struct(new(testDatasetWithMocks), "*"),
	)
	return &testDatasetWithMocks{}
}

type testCodeWithMocks struct {
	*codeComponentImpl
	mocks *Mocks
}

func initializeTestCodeComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testCodeWithMocks {
	wire.Build(
		MockSuperSet, CodeComponentSet,
		wire.Struct(new(testCodeWithMocks), "*"),
	)
	return &testCodeWithMocks{}
}

type testMultiSyncWithMocks struct {
	*multiSyncComponentImpl
	mocks *Mocks
}

func initializeTestMultiSyncComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMultiSyncWithMocks {
	wire.Build(
		MockSuperSet, MultiSyncComponentSet,
		wire.Struct(new(testMultiSyncWithMocks), "*"),
	)
	return &testMultiSyncWithMocks{}
}

type testInternalWithMocks struct {
	*internalComponentImpl
	mocks *Mocks
}

func initializeTestInternalComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testInternalWithMocks {
	wire.Build(
		MockSuperSet, InternalComponentSet,
		wire.Struct(new(testInternalWithMocks), "*"),
	)
	return &testInternalWithMocks{}
}

type testMirrorSourceWithMocks struct {
	*mirrorSourceComponentImpl
	mocks *Mocks
}

func initializeTestMirrorSourceComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMirrorSourceWithMocks {
	wire.Build(
		MockSuperSet, MirrorSourceComponentSet,
		wire.Struct(new(testMirrorSourceWithMocks), "*"),
	)
	return &testMirrorSourceWithMocks{}
}

type testSpaceResourceWithMocks struct {
	*spaceResourceComponentImpl
	mocks *Mocks
}

func initializeTestSpaceResourceComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSpaceResourceWithMocks {
	wire.Build(
		MockSuperSet, SpaceResourceComponentSet,
		wire.Struct(new(testSpaceResourceWithMocks), "*"),
	)
	return &testSpaceResourceWithMocks{}
}

type testTagWithMocks struct {
	*tagComponentImpl
	mocks *Mocks
}

func initializeTestTagComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testTagWithMocks {
	wire.Build(
		MockSuperSet, TagComponentSet,
		wire.Struct(new(testTagWithMocks), "*"),
	)
	return &testTagWithMocks{}
}

type testRecomWithMocks struct {
	*recomComponentImpl
	mocks *Mocks
}

func initializeTestRecomComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRecomWithMocks {
	wire.Build(
		MockSuperSet, RecomComponentSet,
		wire.Struct(new(testRecomWithMocks), "*"),
	)
	return &testRecomWithMocks{}
}

type testSpaceSdkWithMocks struct {
	*spaceSdkComponentImpl
	mocks *Mocks
}

func initializeTestSpaceSdkComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSpaceSdkWithMocks {
	wire.Build(
		MockSuperSet, SpaceSdkComponentSet,
		wire.Struct(new(testSpaceSdkWithMocks), "*"),
	)
	return &testSpaceSdkWithMocks{}
}

type testTelemetryWithMocks struct {
	*telemetryComponentImpl
	mocks *Mocks
}

func initializeTestTelemetryComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testTelemetryWithMocks {
	wire.Build(
		MockSuperSet, TelemetryComponentSet,
		wire.Struct(new(testTelemetryWithMocks), "*"),
	)
	return &testTelemetryWithMocks{}
}

type testClusterWithMocks struct {
	*clusterComponentImpl
	mocks *Mocks
}

func initializeTestClusterComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testClusterWithMocks {
	wire.Build(
		MockSuperSet, ClusterComponentSet,
		wire.Struct(new(testClusterWithMocks), "*"),
	)
	return &testClusterWithMocks{}
}

type testEvaluationWithMocks struct {
	*evaluationComponentImpl
	mocks *Mocks
}

func initializeTestEvaluationComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testEvaluationWithMocks {
	wire.Build(
		MockSuperSet, EvaluationComponentSet,
		wire.Struct(new(testEvaluationWithMocks), "*"),
	)
	return &testEvaluationWithMocks{}
}

type testHFDatasetWithMocks struct {
	*hFDatasetComponentImpl
	mocks *Mocks
}

func initializeTestHFDatasetComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testHFDatasetWithMocks {
	wire.Build(
		MockSuperSet, HFDatasetComponentSet,
		wire.Struct(new(testHFDatasetWithMocks), "*"),
	)
	return &testHFDatasetWithMocks{}
}

type testRepoFileWithMocks struct {
	*repoFileComponentImpl
	mocks *Mocks
}

func initializeTestRepoFileComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRepoFileWithMocks {
	wire.Build(
		MockSuperSet, RepoFileComponentSet,
		wire.Struct(new(testRepoFileWithMocks), "*"),
	)
	return &testRepoFileWithMocks{}
}

type testSensitiveWithMocks struct {
	*sensitiveComponentImpl
	mocks *Mocks
}

func initializeTestSensitiveComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSensitiveWithMocks {
	wire.Build(
		MockSuperSet, SensitiveComponentSet,
		wire.Struct(new(testSensitiveWithMocks), "*"),
	)
	return &testSensitiveWithMocks{}
}

type testSSHKeyWithMocks struct {
	*sSHKeyComponentImpl
	mocks *Mocks
}

func initializeTestSSHKeyComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSSHKeyWithMocks {
	wire.Build(
		MockSuperSet, SSHKeyComponentSet,
		wire.Struct(new(testSSHKeyWithMocks), "*"),
	)
	return &testSSHKeyWithMocks{}
}

type testListWithMocks struct {
	*listComponentImpl
	mocks *Mocks
}

func initializeTestListComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testListWithMocks {
	wire.Build(
		MockSuperSet, ListComponentSet,
		wire.Struct(new(testListWithMocks), "*"),
	)
	return &testListWithMocks{}
}

type testSyncClientSettingWithMocks struct {
	*syncClientSettingComponentImpl
	mocks *Mocks
}

func initializeTestSyncClientSettingComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSyncClientSettingWithMocks {
	wire.Build(
		MockSuperSet, SyncClientSettingComponentSet,
		wire.Struct(new(testSyncClientSettingWithMocks), "*"),
	)
	return &testSyncClientSettingWithMocks{}
}

type testEventWithMocks struct {
	*eventComponentImpl
	mocks *Mocks
}

func initializeTestEventComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testEventWithMocks {
	wire.Build(
		MockSuperSet, EventComponentSet,
		wire.Struct(new(testEventWithMocks), "*"),
	)
	return &testEventWithMocks{}
}

type testLicenseWithMocks struct {
	*licenseComponentImpl
	mocks *Mocks
}

func initializeTestLicenseComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testLicenseWithMocks {
	wire.Build(
		MockSuperSet, LicenseComponentSet,
		wire.Struct(new(testLicenseWithMocks), "*"),
	)
	return &testLicenseWithMocks{}
}

type testImportWithMocks struct {
	*importComponentImpl
	mocks *Mocks
}

func initializeTestImportComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testImportWithMocks {
	wire.Build(
		MockSuperSet, ImportComponentSet,
		wire.Struct(new(testImportWithMocks), "*"),
	)
	return &testImportWithMocks{}
}

type testSpaceTemplateWithMocks struct {
	*spaceTemplateComponentImpl
	mocks *Mocks
}

func initializeTestSpaceTemplateComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSpaceTemplateWithMocks {
	wire.Build(
		MockSuperSet, SpaceTemplateComponentSet,
		wire.Struct(new(testSpaceTemplateWithMocks), "*"),
	)
	return &testSpaceTemplateWithMocks{}
}

type testRuleWithMocks struct {
	*ruleComponentImpl
	mocks *Mocks
}

func initializeTestRuleComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRuleWithMocks {
	wire.Build(
		MockSuperSet, RuleComponentSet,
		wire.Struct(new(testRuleWithMocks), "*"),
	)
	return &testRuleWithMocks{}
}

type testMCPServerWithMocks struct {
	*mcpServerComponentImpl
	mocks *Mocks
}

func initializeTestMCPServerComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMCPServerWithMocks {
	wire.Build(
		MockSuperSet, MCPServerComponentSet,
		wire.Struct(new(testMCPServerWithMocks), "*"),
	)
	return &testMCPServerWithMocks{}
}

type testMCPScannerWithMocks struct {
	*mcpScannerComponentImpl
	mocks *Mocks
}

func initializeTestMCPScannerComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMCPScannerWithMocks {
	wire.Build(
		MockSuperSet, MCPScannerComponentSet,
		wire.Struct(new(testMCPScannerWithMocks), "*"),
	)
	return &testMCPScannerWithMocks{}
}

type testStatComponentWithMocks struct {
	*statComponentImpl
	mocks *Mocks
}

func initializeTestStatComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testStatComponentWithMocks {
	wire.Build(
		MockSuperSet,
		StatComponentTestSet,
		wire.Struct(new(testStatComponentWithMocks), "*"),
	)
	return &testStatComponentWithMocks{}
}

type testLLMServiceComponentWithMocks struct {
	*llmServiceComponentImpl
	mocks *Mocks
}

func initializeTestLLMServiceComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testLLMServiceComponentWithMocks {
	wire.Build(
		MockSuperSet,
		LLMServiceComponentTestSet,
		wire.Struct(new(testLLMServiceComponentWithMocks), "*"),
	)
	return &testLLMServiceComponentWithMocks{}
}

type testMirrorNamespaceMappingWithMocks struct {
	*mirrorNamespaceMappingComponentImpl
	mocks *Mocks
}

type testNotebookWithMocks struct {
	*notebookComponentImpl
	mocks *Mocks
}

func initializeTestNotebookComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testNotebookWithMocks {
	wire.Build(
		MockSuperSet, NotebookComponentSet,
		wire.Struct(new(testNotebookWithMocks), "*"),
	)
	return &testNotebookWithMocks{}
}

func initializeTestMirrorNamespaceMappingComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMirrorNamespaceMappingWithMocks {
	wire.Build(
		MockSuperSet, MirrorNamespaceMappingComponentTestSet,
		wire.Struct(new(testMirrorNamespaceMappingWithMocks), "*"),
	)
	return &testMirrorNamespaceMappingWithMocks{}
}

var MirrorNamespaceMappingComponentTestSet = wire.NewSet(NewTestMirrorNamespaceMappingComponent)

type testXnetWithMocks struct {
	*XnetComponentImpl
	mocks *Mocks
}

func initializeTestXnetComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testXnetWithMocks {
	wire.Build(
		MockSuperSet, XnetComponentSet,
		wire.Struct(new(testXnetWithMocks), "*"),
	)
	return &testXnetWithMocks{}
}
