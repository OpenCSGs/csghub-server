package component

import (
	"context"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewClusterComponent(config *config.Config) (*ClusterComponent, error) {
	c := &ClusterComponent{}
	c.deployer = deploy.NewDeployer()

	return c, nil
}

type ClusterComponent struct {
	deployer deploy.Deployer
}

func (c *ClusterComponent) Index(ctx context.Context) ([]types.ClusterRes, error) {
	return c.deployer.ListCluster(ctx)
}

func (c *ClusterComponent) Update(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	return c.deployer.UpdateCluster(ctx, data)
}
