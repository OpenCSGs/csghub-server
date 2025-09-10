//go:build saas

package component

import (
	"context"
	"fmt"
	"strconv"

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
	c.userResourcesStore = database.NewUserResourcesStore()
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
	userResourcesStore  database.UserResourcesStore
	serverBaseUrl       string
	userLikesStore      database.UserLikesStore
	config              *config.Config
	userSvcClient       rpc.UserSvcClient
	deployTaskStore     database.DeployTaskStore
	recomStore          database.RecomStore
	templateStore       database.SpaceTemplateStore
}

func (c *spaceComponentImpl) checkResourcePurchasableForUpdate(ctx context.Context, req types.UpdateSpaceReq, resource *database.SpaceResource) error {
	priceData, err := c.accountingComponent.QueryPricesBySKUType("", types.AcctPriceListReq{
		SkuType:    types.SKUCSGHub,
		SkuKind:    strconv.Itoa(int(types.SKUPayAsYouGo)),
		ResourceID: strconv.FormatInt(int64(resource.ID), 10),
		Per:        1,
		Page:       1,
	})
	if err != nil {
		return fmt.Errorf("fail to find price data, %w", err)
	}
	if priceData.Total == 0 {
		return fmt.Errorf("cannot find valid price data")
	}
	if priceData.Prices[0].SkuPrice > 0 {
		// check balance
		account, err := c.accountingComponent.QueryBalanceByUserIDInternal(ctx, req.Username)
		if err != nil {
			return fmt.Errorf("cannot find user balance, %w", err)
		}
		if account.Balance <= 0 {
			return fmt.Errorf("balance is not enough to run fee resources. current balance: %f", account.Balance)
		}
	}
	return nil
}

func (c *spaceComponentImpl) updateSpaceByReq(space database.Space, req types.CreateSpaceReq) database.Space {
	space.OrderDetailID = req.OrderDetailID
	return space
}

func (c *spaceComponentImpl) resourceReserved(space *database.Space, req *types.UpdateSpaceReq) bool {
	return space.OrderDetailID != 0 && req.ResourceID != nil
}

func (c *spaceComponentImpl) updateDeployRepoBySpace(repo types.DeployRepo, space *database.Space) types.DeployRepo {
	repo.OrderDetailID = space.OrderDetailID
	return repo
}

func (c *spaceComponentImpl) updateDeployRepoByDeploy(repo types.DeployRepo, deploy *database.Deploy) types.DeployRepo {
	repo.OrderDetailID = deploy.OrderDetailID
	return repo
}

func (c *spaceComponentImpl) addOpWeightToSpaces(ctx context.Context, repoIDs []int64, spaces []*types.Space) {
	opWeights, err := c.recomStore.LoadRepoOpWeights(ctx, repoIDs)
	if err == nil {
		for _, s := range spaces {
			if weight, ok := opWeights[s.RepositoryID]; ok {
				s.RecomOpWeight = weight
			}
		}
	}
}
