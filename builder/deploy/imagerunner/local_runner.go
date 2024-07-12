package imagerunner

import (
	"context"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/types"
)

var _ Runner = (*LocalRunner)(nil)

// Typically this is for local test only
type LocalRunner struct{}

// InstanceLogs implements Runner.
func (r *LocalRunner) InstanceLogs(context.Context, *types.InstanceLogsRequest) (<-chan string, error) {
	output := make(chan string, 1)
	output <- "test build log"
	return output, nil
}

// GetReplica implements Runner.
func (r *LocalRunner) GetReplica(context.Context, *types.StatusRequest) (*types.ReplicaResponse, error) {
	return &types.ReplicaResponse{
		Code:           1,
		Message:        "success",
		ActualReplica:  0,
		DesiredReplica: 0,
		Instances:      []types.Instance{},
	}, nil
}

// Exist implements Runner.
func (r *LocalRunner) Exist(context.Context, *types.CheckRequest) (*types.StatusResponse, error) {
	return &types.StatusResponse{
		Code:    1,
		Message: "deploy exist",
	}, nil
}

func NewLocalRunner() Runner {
	return &LocalRunner{}
}

func (r *LocalRunner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	return &types.RunResponse{
		Code:    0,
		Message: "deploy scheduled",
	}, nil
}

func (r *LocalRunner) Status(ctx context.Context, req *types.StatusRequest) (*types.StatusResponse, error) {
	return &types.StatusResponse{
		Code:    common.Running,
		Message: "deploy success",
	}, nil
}

func (r *LocalRunner) StatusAll(ctx context.Context) (map[string]types.StatusResponse, error) {
	status := make(map[string]types.StatusResponse)
	status["gradio-test-app"] = types.StatusResponse{Code: 21}
	status["gradio-test-app-v1-0"] = types.StatusResponse{Code: 20}
	status["image-123"] = types.StatusResponse{Code: 25}
	return status, nil
}

func (r *LocalRunner) Logs(ctx context.Context, req *types.LogsRequest) (<-chan string, error) {
	output := make(chan string, 1)
	output <- "test build log"
	return output, nil
}

func (r *LocalRunner) Stop(ctx context.Context, req *types.StopRequest) (*types.StopResponse, error) {
	return &types.StopResponse{}, nil
}

func (r *LocalRunner) Purge(ctx context.Context, req *types.PurgeRequest) (*types.PurgeResponse, error) {
	return nil, nil
}

func (h *LocalRunner) ListCluster(ctx context.Context) ([]types.ClusterResponse, error) {
	return nil, nil
}

func (h *LocalRunner) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterResponse, error) {
	return nil, nil
}

func (h *LocalRunner) UpdateCluster(ctx context.Context, data *types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	return nil, nil
}
