package imagerunner

import (
	"context"

	"opencsg.com/csghub-server/builder/deploy/common"
)

var _ Runner = (*LocalRunner)(nil)

// Typically this is for local test only
type LocalRunner struct{}

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
