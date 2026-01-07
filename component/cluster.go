package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type ClusterComponent interface {
	Index(ctx context.Context) ([]types.ClusterRes, error)
	GetClusterWithResourceByID(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	Update(ctx context.Context, data types.ClusterRequest) (*types.ClusterRes, error)
	GetClusterUsages(ctx context.Context) ([]types.ClusterRes, error)
	GetDeploys(ctx context.Context, req types.DeployReq) ([]types.DeployRes, int, error)
	GetClusterByID(ctx context.Context, clusterId string) (*database.ClusterInfo, error)
}

func NewClusterComponent(config *config.Config) (ClusterComponent, error) {
	c := &clusterComponentImpl{}
	c.deployer = deploy.NewDeployer()
	c.clusterStore = database.NewClusterInfoStore()

	c.deployTaskStore = database.NewDeployTaskStore()
	acctClient, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	c.acctClient = acctClient
	return c, nil
}

type clusterComponentImpl struct {
	deployer        deploy.Deployer
	clusterStore    database.ClusterInfoStore
	deployTaskStore database.DeployTaskStore
	acctClient      accounting.AccountingClient
}

func (c *clusterComponentImpl) Index(ctx context.Context) ([]types.ClusterRes, error) {
	clusterInos, err := c.clusterStore.List(ctx)
	if err != nil {
		return nil, err
	}
	var clusters []types.ClusterRes
	for _, clusterInfo := range clusterInos {
		if types.ClusterStatus(clusterInfo.Status) == types.ClusterStatusUnavailable {
			continue
		}
		if !clusterInfo.Enable {
			continue
		}
		cluster := &types.ClusterRes{
			ClusterID:    clusterInfo.ClusterID,
			Region:       clusterInfo.Region,
			Zone:         clusterInfo.Zone,
			Provider:     clusterInfo.Provider,
			StorageClass: clusterInfo.StorageClass,
			Status:       clusterInfo.Status,
			Endpoint:     clusterInfo.RunnerEndpoint,
		}
		clusters = append(clusters, *cluster)
	}
	return clusters, nil
}

func (c *clusterComponentImpl) GetClusterUsages(ctx context.Context) ([]types.ClusterRes, error) {
	clusterList, err := c.deployer.ListCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("list clusters failed: %w", err)
	}

	if len(clusterList) == 0 {
		return []types.ClusterRes{}, nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	resultMap := make(map[string]*types.ClusterRes, len(clusterList))
	var mu sync.Mutex

	for i := range clusterList {
		cluster := clusterList[i]
		g.Go(func() error {
			if groupCtx.Err() != nil {
				mu.Lock()
				resultMap[cluster.ClusterID] = &types.ClusterRes{
					ClusterID: cluster.ClusterID,
					Status:    types.ClusterStatusUnavailable,
					Region:    cluster.Region,
					Zone:      cluster.Zone,
					Provider:  cluster.Provider,
				}
				mu.Unlock()
				return nil
			}
			res, err := c.deployer.GetClusterUsageById(groupCtx, cluster.ClusterID)
			if err != nil {
				slog.Error("get cluster usage failed",
					"clusterID", cluster.ClusterID,
					"error", err)
				res = &types.ClusterRes{
					ClusterID: cluster.ClusterID,
					Status:    types.ClusterStatusUnavailable,
					Region:    cluster.Region,
					Zone:      cluster.Zone,
					Provider:  cluster.Provider,
					GPUUsage:  0,
					MemUsage:  0,
					CPUUsage:  0,
				}
			}

			res.LastUpdateTime = cluster.LastUpdateTime

			mu.Lock()
			resultMap[cluster.ClusterID] = res
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := make([]types.ClusterRes, 0, len(clusterList))
	for _, cluster := range clusterList {
		if res, ok := resultMap[cluster.ClusterID]; ok {
			result = append(result, *res)
		} else {
			result = append(result, types.ClusterRes{
				ClusterID: cluster.ClusterID,
				Status:    types.ClusterStatusUnavailable,
				Region:    cluster.Region,
				Zone:      cluster.Zone,
				Provider:  cluster.Provider,
			})
		}
	}

	return result, nil
}

func (c *clusterComponentImpl) GetDeploys(ctx context.Context, req types.DeployReq) ([]types.DeployRes, int, error) {
	deploys, total, err := c.deployTaskStore.ListDeployByType(ctx, req)
	if err != nil {
		slog.Error("Failed to get deploys", slog.Any("error", err))
		return nil, 0, err
	}
	clusterInos, err := c.clusterStore.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	var res []types.DeployRes
	for _, deploy := range deploys {
		if deploy.User == nil {
			continue
		}
		totalTime := 0
		totalFee := 0
		scene := types.SceneModelInference
		switch deploy.Type {
		case types.FinetuneType:
			scene = types.SceneModelFinetune
		case types.SpaceType:
			scene = types.SceneSpace
		case types.InferenceType:
			scene = types.SceneModelInference
		default:
			slog.Debug("ignore invalid deploy type", slog.Any("scene", scene))
			continue
		}
		req2 := types.ActStatementsReq{
			Scene:        int(scene),
			UserUUID:     deploy.UserUUID,
			StartTime:    deploy.CreatedAt.Format(time.DateTime),
			EndTime:      time.Now().Format(time.DateTime),
			InstanceName: deploy.SvcName,
			Per:          1,
			Page:         1,
		}
		stat, _ := c.acctClient.ListStatementByUserIDAndTime(req2)
		if stat != nil {
			tempJSON, err := json.Marshal(stat)
			if err != nil {
				return nil, 0, fmt.Errorf("error to marshal json, %w", err)
			}
			var statResult *types.AcctStatementsResult
			if err := json.Unmarshal(tempJSON, &statResult); err != nil {
				return nil, 0, fmt.Errorf("error to unmarshal json, %w", err)
			}
			totalTime = statResult.Total
			totalFee = int(math.Abs(statResult.TotalValue))
		}
		// get cluster region
		region := getClusterRegion(deploy.ClusterID, clusterInos)

		res = append(res, types.DeployRes{
			ClusterID:     deploy.ClusterID,
			ClusterRegion: region,
			DeployName:    deploy.DeployName,
			Status:        deployStatusCodeToString(deploy.Status),
			CreateTime:    deploy.CreatedAt,
			User: types.User{
				ID:       deploy.UserID,
				Username: deploy.User.Username,
				Avatar:   deploy.User.Avatar,
			},
			Resource:        deploy.Hardware,
			TotalTimeInMin:  totalTime,
			TotalFeeInCents: totalFee,
			SvcName:         deploy.SvcName,
		})
	}
	return res, total, nil
}

func (c *clusterComponentImpl) GetClusterWithResourceByID(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	clusterInfo, err := c.clusterStore.ByClusterID(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	res, err := c.deployer.GetClusterById(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	res.ClusterID = clusterInfo.ClusterID
	res.StorageClass = clusterInfo.StorageClass
	res.Region = clusterInfo.Region
	res.Zone = clusterInfo.Zone
	res.Provider = clusterInfo.Provider
	res.Status = clusterInfo.Status
	res.Endpoint = clusterInfo.RunnerEndpoint
	return res, nil
}

func (c *clusterComponentImpl) Update(ctx context.Context, data types.ClusterRequest) (*types.ClusterRes, error) {
	clusterInfo, err := c.clusterStore.ByClusterID(ctx, data.ClusterID)
	if err != nil {
		return nil, err
	}
	clusterInfo.StorageClass = data.StorageClass
	clusterInfo.Region = data.Region
	clusterInfo.Zone = data.Zone
	clusterInfo.Provider = data.Provider
	err = c.clusterStore.Update(ctx, clusterInfo)
	if err != nil {
		return nil, err
	}
	var clusterRes types.ClusterRes
	clusterRes.ClusterID = clusterInfo.ClusterID
	clusterRes.StorageClass = clusterInfo.StorageClass
	clusterRes.Region = clusterInfo.Region
	clusterRes.Zone = clusterInfo.Zone
	clusterRes.Provider = clusterInfo.Provider
	clusterRes.Status = clusterInfo.Status
	clusterRes.Endpoint = clusterInfo.RunnerEndpoint
	return &clusterRes, nil

}

func getClusterRegion(clusterId string, clusterInos []database.ClusterInfo) string {
	if len(clusterInos) == 0 {
		return "unknown"
	}
	for _, cluster := range clusterInos {
		if cluster.ClusterID == clusterId {
			return cluster.Region
		}
	}
	return clusterInos[0].Region
}

func (c *clusterComponentImpl) GetClusterByID(ctx context.Context, clusterId string) (*database.ClusterInfo, error) {
	clusterInfo, err := c.clusterStore.ByClusterID(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	return &clusterInfo, nil
}

func ExtractDeployTargetAndHost(ctx context.Context, clusterComp ClusterComponent, req types.EndpointReq) (string, string, error) {
	target := req.Target
	host := req.Host
	appSvcName := req.SvcName

	cluster, err := clusterComp.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get cluster by id %s, error: %w", req.ClusterID, err)
	}

	if len(cluster.AppEndpoint) < 1 {
		slog.Warn("app endpoint of cluster is empty", slog.Any("clusterID", cluster.ClusterID))
		return target, host, nil
	}

	target = cluster.AppEndpoint
	if len(req.Endpoint) < 1 {
		return "", "", fmt.Errorf("endpoint of deploy %s is empty", appSvcName)
	}

	host, err = extractHostFromEndpoint(req.Endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract host from endpoint %s, error: %w", req.Endpoint, err)
	}

	return target, host, nil
}

func extractHostFromEndpoint(endpoint string) (string, error) {
	// http://u-neo888-test0922-2-lv.spaces.app.internal
	// extract host from url
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint url %s, error: %w", endpoint, err)
	}
	host := u.Hostname()
	if len(host) < 1 {
		return "", fmt.Errorf("extract host of endpoint %s is empty", endpoint)
	}
	return host, nil
}
