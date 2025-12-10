//go:build !saas && !ee

package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceComponent(config *config.Config) (SpaceComponent, error) {
	c := &spaceComponentImpl{}
	c.spaceStore = database.NewSpaceStore()
	var err error
	c.spaceSdkStore = database.NewSpaceSdkStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.repoStore = database.NewRepoStore()
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	c.deployer = deploy.NewDeployer()
	c.publicRootDomain = config.Space.PublicRootDomain
	c.userStore = database.NewUserStore()
	c.accountingComponent, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.serverBaseUrl = config.APIServer.PublicDomain
	c.userLikesStore = database.NewUserLikesStore()
	c.config = config
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))

	c.deployTaskStore = database.NewDeployTaskStore()
	c.recomStore = database.NewRecomStore()
	c.git, err = git.NewGitServer(config)
	if err != nil {
		return nil, err
	}
	c.templateStore = database.NewSpaceTemplateStore()
	return c, nil
}

type spaceComponentImpl struct {
	repoComponent       RepoComponent
	git                 gitserver.GitServer
	spaceStore          database.SpaceStore
	spaceSdkStore       database.SpaceSdkStore
	spaceResourceStore  database.SpaceResourceStore
	repoStore           database.RepoStore
	userStore           database.UserStore
	deployer            deploy.Deployer
	publicRootDomain    string
	accountingComponent AccountingComponent
	serverBaseUrl       string
	userLikesStore      database.UserLikesStore
	config              *config.Config
	userSvcClient       rpc.UserSvcClient
	deployTaskStore     database.DeployTaskStore
	recomStore          database.RecomStore
	templateStore       database.SpaceTemplateStore
}

func (c *spaceComponentImpl) checkResourcePurchasableForUpdate(ctx context.Context, req types.UpdateSpaceReq, resource *database.SpaceResource) error {
	return nil
}

func (c *spaceComponentImpl) updateSpaceByReq(space database.Space, req types.CreateSpaceReq) database.Space {
	return space
}

func (c *spaceComponentImpl) resourceReserved(space *database.Space, req *types.UpdateSpaceReq) bool {
	return false
}

func (c *spaceComponentImpl) updateDeployRepoBySpace(repo types.DeployRepo, space *database.Space) types.DeployRepo {
	repo.MinReplica = space.MinReplica
	return repo
}

func (c *spaceComponentImpl) updateDeployRepoByDeploy(repo types.DeployRepo, deploy *database.Deploy) types.DeployRepo {
	return repo
}

func (c *spaceComponentImpl) addOpWeightToSpaces(ctx context.Context, repoIDs []int64, spaces []*types.Space) {
}

func (c *spaceComponentImpl) syncCodeAgentIfExists(_ string, _ string, _ string, _ types.CodeAgentSyncOperation) {
}
