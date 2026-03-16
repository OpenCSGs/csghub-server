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

func (c *clusterComponentImpl) SetClusterNodeAccessMode(ctx context.Context, req types.SetNodeAccessModeReq) error {
	return nil
}

func (c *clusterComponentImpl) CheckExclusiveResource(ctx context.Context, req types.CheckExclusiveReq) (*types.CheckExclusiveResp, error) {
	return &types.CheckExclusiveResp{UsedExclusive: false}, nil
}

func (c *clusterComponentImpl) GetDeploysByTimeRange(ctx context.Context, req types.DeployTimeRangeReq) ([]database.Deploy, int, error) {
	return nil, 0, nil
}

func (c *clusterComponentImpl) GetDeploysByTimeRangeStream(ctx context.Context, req types.DeployTimeRangeReq) (<-chan []string, <-chan error) {
	deployCh := make(chan []string)
	errCh := make(chan error, 1)

	go func() {
		defer close(deployCh)
		defer close(errCh)
	}()

	return deployCh, errCh
}

func (c *clusterComponentImpl) GetWorkflowsByTimeRange(ctx context.Context, req types.WorkflowTimeRangeReq) ([]database.ArgoWorkflow, int, error) {
	return nil, 0, nil
}

func (c *clusterComponentImpl) GetWorkflowsByTimeRangeStream(ctx context.Context, req types.WorkflowTimeRangeReq) (<-chan []string, <-chan error) {
	workflowCh := make(chan []string)
	errCh := make(chan error, 1)

	go func() {
		defer close(workflowCh)
		defer close(errCh)
	}()

	return workflowCh, errCh
}
