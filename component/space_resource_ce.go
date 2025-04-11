//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceResourceComponent(config *config.Config) (SpaceResourceComponent, error) {
	c := &spaceResourceComponentImpl{}
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.deployer = deploy.NewDeployer()
	c.userStore = database.NewUserStore()
	ac, err := NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.accountComponent = ac
	return c, nil
}

type spaceResourceComponentImpl struct {
	spaceResourceStore database.SpaceResourceStore
	deployer           deploy.Deployer
	userStore          database.UserStore
	accountComponent   AccountingComponent
}

func (c *spaceResourceComponentImpl) updatePriceInfo(currentUser string, resources []types.SpaceResource) error {
	return nil

}

func (c *spaceResourceComponentImpl) appendUserResources(ctx context.Context, currentUser string, clusterID string, resources []types.SpaceResource) ([]types.SpaceResource, error) {
	return resources, nil
}

func (c *spaceResourceComponentImpl) deployAvailable(deployType int, hardware types.HardWare) bool {
	if deployType == types.FinetuneType {
		if hardware.Gpu.Num == "" {
			return false
		}
	}
	return true
}
