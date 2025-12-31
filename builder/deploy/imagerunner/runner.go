package imagerunner

import (
	"context"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

type Runner interface {
	Run(context.Context, *types.RunRequest) (*types.RunResponse, error)
	Stop(context.Context, *types.StopRequest) (*types.StopResponse, error)
	Purge(context.Context, *types.PurgeRequest) (*types.PurgeResponse, error)
	Status(context.Context, *types.StatusRequest) (*types.StatusResponse, error)
	Logs(context.Context, *types.LogsRequest) (<-chan string, error)
	Exist(context.Context, *types.CheckRequest) (*types.StatusResponse, error)
	GetReplica(context.Context, *types.StatusRequest) (*types.ReplicaResponse, error)
	InstanceLogs(context.Context, *types.InstanceLogsRequest) (<-chan string, error)
	ListCluster(ctx context.Context) ([]types.ClusterRes, error)
	GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	UpdateCluster(ctx context.Context, data *types.ClusterRequest) (*types.UpdateClusterResponse, error)
	SubmitWorkFlow(context.Context, *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error)
	DeleteWorkFlow(context.Context, types.ArgoWorkFlowDeleteReq) (*httpbase.R, error)
	GetWorkFlow(context.Context, types.ArgoWorkFlowDeleteReq) (*types.ArgoWorkFlowRes, error)
	SubmitFinetuneJob(context.Context, *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error)
	SetVersionsTraffic(ctx context.Context, clusterID, svcName string, req []types.TrafficReq) error
	CreateRevisions(context.Context, *types.CreateRevisionReq) error
	ListKsvcVersions(ctx context.Context, clusterID, svcName string) ([]types.KsvcRevisionInfo, error)
	DeleteKsvcVersion(ctx context.Context, clusterID, svcName, commitID string) error
}
