package imagerunner

var _ Runner = (*LocalRunner)(nil)

// Typically this is for local test only
type LocalRunner struct{}

func (r *LocalRunner) Run(req *RunRequest) (*RunResponse, error) {
	return &RunResponse{}, nil
}

func (r *LocalRunner) Status(req *StatusRequest) (*StatusResponse, error) {
	return &StatusResponse{}, nil
}

func (r *LocalRunner) Logs(req *LogsRequest) (*LogsResponse, error) {
	return &LogsResponse{}, nil
}
