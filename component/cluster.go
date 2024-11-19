package component

import (
	"context"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type ClusterComponent interface {
	Index(ctx context.Context) ([]types.ClusterRes, error)
	GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	Update(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error)
}

func NewClusterComponent(config *config.Config) (ClusterComponent, error) {
	c := &clusterComponentImpl{}
	c.deployer = deploy.NewDeployer()

	return c, nil
}

type clusterComponentImpl struct {
	deployer deploy.Deployer
}

func (c *clusterComponentImpl) Index(ctx context.Context) ([]types.ClusterRes, error) {
	return c.deployer.ListCluster(ctx)
}

func (c *clusterComponentImpl) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	return c.deployer.GetClusterById(ctx, clusterId)
}

func (c *clusterComponentImpl) Update(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	return c.deployer.UpdateCluster(ctx, data)
}
