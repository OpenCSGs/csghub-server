//go:build !saas && !ee

package component

import (
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
)

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

func NewTestSpaceResourceComponent(config *config.Config, stores *tests.MockStores, deployer deploy.Deployer, accountComponent AccountingComponent) *spaceResourceComponentImpl {
	return &spaceResourceComponentImpl{
		spaceResourceStore: stores.SpaceResource,
		deployer:           deployer,
		userStore:          stores.User,
		accountComponent:   accountComponent,
	}
}

func NewTestMirrorNamespaceMappingComponent(config *config.Config, stores *tests.MockStores) *mirrorNamespaceMappingComponentImpl {
	return &mirrorNamespaceMappingComponentImpl{
		mirrorNamespaceMappingStore: stores.MirrorNamespaceMapping,
	}
}
