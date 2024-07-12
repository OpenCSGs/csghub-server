package imagerunner

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type Runner interface {
	Run(context.Context, *RunRequest) (*RunResponse, error)
	Stop(context.Context, *StopRequest) (*StopResponse, error)
	Status(context.Context, *StatusRequest) (*StatusResponse, error)
	StatusAll(context.Context) (map[string]types.StatusResponse, error)
	Logs(context.Context, *LogsRequest) (<-chan string, error)
	Exist(context.Context, *CheckRequest) (*StatusResponse, error)
	GetReplica(context.Context, *StatusRequest) (*ReplicaResponse, error)
	InstanceLogs(context.Context, *InstanceLogsRequest) (<-chan string, error)
	ListCluster(ctx context.Context) ([]ClusterResponse, error)
	UpdateCluster(ctx context.Context, data interface{}) (*UpdateClusterResponse, error)
}
