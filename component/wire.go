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

type testDatasetViewerWithMocks struct {
	*datasetViewerComponentImpl
	mocks *Mocks
}

func initializeTestDatasetViewerComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDatasetViewerWithMocks {
	wire.Build(
		MockSuperSet, DatasetViewerComponentSet,
		wire.Struct(new(testDatasetViewerWithMocks), "*"),
	)
	return &testDatasetViewerWithMocks{}
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
