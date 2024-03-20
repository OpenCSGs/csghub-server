package imagerunner

import "context"

type Runner interface {
	Run(context.Context, *RunRequest) (*RunResponse, error)
	Stop(context.Context, *StopRequest) (*StopResponse, error)
	Status(context.Context, *StatusRequest) (*StatusResponse, error)
	Logs(context.Context, *LogsRequest) (*LogsResponse, error)
}
