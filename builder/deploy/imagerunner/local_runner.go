package imagerunner

import "context"

var _ Runner = (*LocalRunner)(nil)

// Typically this is for local test only
type LocalRunner struct{}

func NewLocalRunner() Runner {
	return &LocalRunner{}
}

func (r *LocalRunner) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	return &RunResponse{}, nil
}

func (r *LocalRunner) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	return &StatusResponse{}, nil
}

func (r *LocalRunner) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	return &LogsResponse{}, nil
}
