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
