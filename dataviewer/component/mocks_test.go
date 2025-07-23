package component

import (
	"github.com/google/wire"
	"go.opentelemetry.io/otel"
	mock_accounting "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_mirror "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/mirrorserver"
	mock_preader "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/parquet"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_rsa "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rsa"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/parquet"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	hubCom "opencsg.com/csghub-server/component"
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

type Mocks struct {
	stores            *tests.MockStores
	components        *mockedComponents
	gitServer         *mock_git.MockGitServer
	userSvcClient     *mock_rpc.MockUserSvcClient
	s3Client          *mock_s3.MockClient
	mirrorServer      *mock_mirror.MockMirrorServer
	deployer          *mock_deploy.MockDeployer
	accountingClient  *mock_accounting.MockAccountingClient
	preader           *mock_preader.MockReader
	limitOffsetReader *mock_preader.MockLimitOffsetCountReader
	moderationClient  *mock_rpc.MockModerationSvcClient
	rsaReader         *mock_rsa.MockKeysReader
}

func ProvideTestConfig() *config.Config {
	return &config.Config{}
}

func NewTestDatasetViewerComponent(stores *tests.MockStores, cfg *config.Config, repoComponent hubCom.RepoComponent, gitServer gitserver.GitServer, preader parquet.Reader, limitOffsetReader parquet.LimitOffsetCountReader) *datasetViewerComponentImpl {
	return &datasetViewerComponentImpl{
		cfg:                    cfg,
		repoStore:              stores.Repo,
		repoComponent:          repoComponent,
		gitServer:              gitServer,
		preader:                preader,
		limitOffsetCountReader: limitOffsetReader,
		viewerStore:            stores.ViewerStore,
		tracer:                 otel.Tracer("dataset-viewer"),
	}
}

var DatasetViewerComponentSet = wire.NewSet(NewTestDatasetViewerComponent)
