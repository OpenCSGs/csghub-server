//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *clusterComponentImpl) GetClusterNodes(ctx context.Context) ([]database.ClusterNodeWithRegion, error) {
	return nil, nil
}

func (c *clusterComponentImpl) GetClusterNodeByID(ctx context.Context, id int64) (*database.ClusterNodeWithRegion, error) {
	return nil, nil
}

func (c *clusterComponentImpl) QueryClusterDeploys(ctx context.Context, req types.ClusterDeployReq) ([]database.Deploy, int, error) {
	return nil, 0, nil
}

func (c *clusterComponentImpl) QueryClusterWorkflows(ctx context.Context, req types.ClusterWFReq) ([]database.ArgoWorkflow, int, error) {
	return nil, 0, nil
}

func (c *clusterComponentImpl) UpdateClusterNodeVXPU(ctx context.Context, req types.UpdateClusterNodeReq) (*database.ClusterNodeWithRegion, error) {
	return nil, nil
}
