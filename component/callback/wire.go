//go:build wireinject
// +build wireinject

package callback

import (
	"context"

	"github.com/google/wire"
	"github.com/stretchr/testify/mock"
)

type testSyncVersionGeneratorWithMocks struct {
	*syncVersionGeneratorImpl
	mocks *Mocks
}

func initializeTestSyncVersionGenerator(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSyncVersionGeneratorWithMocks {
	wire.Build(
		MockCallbackSuperSet, SyncVersionGeneratorSet,
		wire.Struct(new(testSyncVersionGeneratorWithMocks), "*"),
	)
	return &testSyncVersionGeneratorWithMocks{}
}

type testGitCallbackWithMocks struct {
	*gitCallbackComponentImpl
	mocks *Mocks
}

func initializeTestGitCallbackComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testGitCallbackWithMocks {
	wire.Build(
		MockCallbackSuperSet, GitCallbackComponentSet,
		wire.Struct(new(testGitCallbackWithMocks), "*"),
	)
	return &testGitCallbackWithMocks{}
}
