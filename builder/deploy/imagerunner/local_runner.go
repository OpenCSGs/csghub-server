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
func (r *LocalRunner) InstanceLogs(context.Context, *InstanceLogsRequest) (<-chan string, error) {
	output := make(chan string, 1)
	output <- "test build log"
	return output, nil
}

// GetReplica implements Runner.
func (r *LocalRunner) GetReplica(context.Context, *StatusRequest) (*ReplicaResponse, error) {
	return &ReplicaResponse{
		Code:           1,
		Message:        "success",
		ActualReplica:  0,
		DesiredReplica: 0,
		Instances:      []types.Instance{},
	}, nil
}

// Exist implements Runner.
func (r *LocalRunner) Exist(context.Context, *CheckRequest) (*StatusResponse, error) {
	return &StatusResponse{
		Code:    1,
		Message: "deploy exist",
	}, nil
}

func NewLocalRunner() Runner {
	return &LocalRunner{}
}

func (r *LocalRunner) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	return &RunResponse{
		Code:    0,
		Message: "deploy scheduled",
	}, nil
}

func (r *LocalRunner) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	return &StatusResponse{
		Code:    common.Running,
		Message: "deploy success",
	}, nil
}

func (r *LocalRunner) StatusAll(ctx context.Context) (map[string]StatusResponse, error) {
	status := make(map[string]StatusResponse)
	status["gradio-test-app"] = StatusResponse{Code: 21}
	status["gradio-test-app-v1-0"] = StatusResponse{Code: 20}
	status["image-123"] = StatusResponse{Code: 25}
	return status, nil
}

func (r *LocalRunner) Logs(ctx context.Context, req *LogsRequest) (<-chan string, error) {
	output := make(chan string, 1)
	output <- "test build log"
	return output, nil
}

func (r *LocalRunner) Stop(ctx context.Context, req *StopRequest) (*StopResponse, error) {
	return &StopResponse{}, nil
}

func (h *LocalRunner) ListCluster(ctx context.Context) ([]ClusterResponse, error) {
	return nil, nil
}

func (h *LocalRunner) UpdateCluster(ctx context.Context, data interface{}) (*UpdateClusterResponse, error) {
	return nil, nil
}
