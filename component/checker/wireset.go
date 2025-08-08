package checker

import (
	"github.com/google/wire"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
)

func NewTestLFSExistsChecker(config *config.Config, stores *tests.MockStores, gitserver gitserver.GitServer, s3Client s3.Client) *LFSExistsChecker {
	return &LFSExistsChecker{
		repoStore: stores.Repo,
		gitServer: gitserver,
		config:    config,
		s3Client:  s3Client,
	}
}

var LFSExistsCheckerTestSet = wire.NewSet(NewTestLFSExistsChecker)

func NewTestFileSizeChecker(config *config.Config, stores *tests.MockStores, gitserver gitserver.GitServer) *FileSizeChecker {
	return &FileSizeChecker{
		repoStore: stores.Repo,
		gitServer: gitserver,
		config:    config,
	}
}

var FileSizeCheckerTestSet = wire.NewSet(NewTestFileSizeChecker)

var MockedStoreSet = wire.NewSet(
	tests.NewMockStores,
)

func ProvideTestConfig() *config.Config {
	return &config.Config{}
}

var MockedGitServerSet = wire.NewSet(
	mock_git.NewMockGitServer,
	wire.Bind(new(gitserver.GitServer), new(*mock_git.MockGitServer)),
)

// var AllMockSet = wire.NewSet(
// 	wire.Struct(new(Mocks), "*"),
// )

var MockedS3Set = wire.NewSet(
	mock_s3.NewMockClient,
	wire.Bind(new(s3.Client), new(*mock_s3.MockClient)),
)
