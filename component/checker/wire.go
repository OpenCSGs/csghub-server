//go:build wireinject
// +build wireinject

package checker

import (
	"context"

	"github.com/google/wire"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/tests"
)

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

type Mocks struct {
	stores    *tests.MockStores
	gitServer *gitserver.MockGitServer
	s3Client  *s3.MockClient
}

type testFileSizeCheckerWithMocks struct {
	*FileSizeChecker
	mocks *Mocks
}

type testLFSExistsCheckerWithMocks struct {
	*LFSExistsChecker
	mocks *Mocks
}

type SkillMocks struct {
	stores    *tests.MockStores
	gitServer *gitserver.MockGitServer
}

type testSkillFileCheckerWithMocks struct {
	*SkillFileChecker
	mocks *SkillMocks
}

func initializeTestFileSizeChecker(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testFileSizeCheckerWithMocks {
	wire.Build(
		ProvideTestConfig,
		MockedStoreSet,
		MockedGitServerSet,
		MockedS3Set,
		FileSizeCheckerTestSet,
		wire.Struct(new(Mocks), "*"),
		wire.Struct(new(testFileSizeCheckerWithMocks), "*"),
	)
	return nil
}

func initializeTestLFSExistsChecker(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testLFSExistsCheckerWithMocks {
	wire.Build(
		ProvideTestConfig,
		MockedStoreSet,
		MockedGitServerSet,
		MockedS3Set,
		LFSExistsCheckerTestSet,
		wire.Struct(new(Mocks), "*"),
		wire.Struct(new(testLFSExistsCheckerWithMocks), "*"),
	)
	return nil
}

func initializeTestSkillFileChecker(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSkillFileCheckerWithMocks {
	wire.Build(
		ProvideTestConfig,
		MockedStoreSet,
		MockedGitServerSet,
		SkillFileCheckerTestSet,
		wire.Struct(new(SkillMocks), "*"),
		wire.Struct(new(testSkillFileCheckerWithMocks), "*"),
	)
	return nil
}
