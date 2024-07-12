package imagerunner

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type Runner interface {
	Run(context.Context, *types.RunRequest) (*types.RunResponse, error)
	Stop(context.Context, *types.StopRequest) (*types.StopResponse, error)
	Purge(context.Context, *types.PurgeRequest) (*types.PurgeResponse, error)
	Status(context.Context, *types.StatusRequest) (*types.StatusResponse, error)
	StatusAll(context.Context) (map[string]types.StatusResponse, error)
	Logs(context.Context, *types.LogsRequest) (<-chan string, error)
	Exist(context.Context, *types.CheckRequest) (*types.StatusResponse, error)
	GetReplica(context.Context, *types.StatusRequest) (*types.ReplicaResponse, error)
	InstanceLogs(context.Context, *types.InstanceLogsRequest) (<-chan string, error)
	ListCluster(ctx context.Context) ([]types.ClusterResponse, error)
	GetClusterById(ctx context.Context, clusterId string) (*types.ClusterResponse, error)
	UpdateCluster(ctx context.Context, data *types.ClusterRequest) (*types.UpdateClusterResponse, error)
}
