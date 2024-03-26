package imagebuilder

import "context"

type Builder interface {
	Build(context.Context, *BuildRequest) (*BuildResponse, error)
	Status(context.Context, *StatusRequest) (*StatusResponse, error)
	Logs(context.Context, *LogsRequest) (<-chan string, error)
}
