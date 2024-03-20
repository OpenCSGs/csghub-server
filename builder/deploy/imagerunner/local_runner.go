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

func (r *LocalRunner) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	return &LogsResponse{}, nil
}

func (r *LocalRunner) Stop(ctx context.Context, req *StopRequest) (*StopResponse, error) {
	return &StopResponse{}, nil
}
