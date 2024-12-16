package callback

import (
	"github.com/google/wire"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/component"
)

type Mocks struct {
	stores               *tests.MockStores
	tagComponent         *mock_component.MockTagComponent
	spaceComponent       *mock_component.MockSpaceComponent
	gitServer            *mock_git.MockGitServer
	runtimeArchComponent *mock_component.MockRuntimeArchitectureComponent
}

var AllMockSet = wire.NewSet(
	wire.Struct(new(Mocks), "*"),
)

var MockCallbackSuperSet = wire.NewSet(
	component.MockedStoreSet, component.MockedComponentSet, AllMockSet,
	component.ProvideTestConfig, component.MockedGitServerSet, component.MockedModerationSvcClientSet,
)

func NewTestGitCallbackComponent(config *config.Config, stores *tests.MockStores, gitServer gitserver.GitServer, tagComponent component.TagComponent, modSvcClient rpc.ModerationSvcClient, runtimeArchComponent component.RuntimeArchitectureComponent, spaceComponent component.SpaceComponent) *gitCallbackComponentImpl {
	return &gitCallbackComponentImpl{
		config:                    config,
		gitServer:                 gitServer,
		tagComponent:              tagComponent,
		modSvcClient:              modSvcClient,
		modelStore:                stores.Model,
		datasetStore:              stores.Dataset,
		spaceComponent:            spaceComponent,
		spaceStore:                stores.Space,
		repoStore:                 stores.Repo,
		repoRelationStore:         stores.RepoRelation,
		mirrorStore:               stores.Mirror,
		repoRuntimeFrameworkStore: stores.RepoRuntimeFramework,
		runtimeArchComponent:      runtimeArchComponent,
		runtimeArchStore:          stores.RuntimeArch,
		runtimeFrameworkStore:     stores.RuntimeFramework,
		tagStore:                  stores.Tag,
		tagRuleStore:              stores.TagRule,
	}
}

var GitCallbackComponentSet = wire.NewSet(NewTestGitCallbackComponent)
