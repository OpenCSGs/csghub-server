package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/url"
	"time"

	"opencsg.com/csghub-server/builder/rpc"

	units "github.com/dustin/go-humanize"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type ClusterComponent interface {
	Index(ctx context.Context) ([]types.ClusterRes, error)
	IndexPublic(ctx context.Context) (types.PublicClusterRes, error)
	GetClusterWithResourceByID(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	Update(ctx context.Context, data types.ClusterRequest) (*types.ClusterRes, error)
	GetClusterUsages(ctx context.Context) ([]types.ClusterRes, error)
	GetDeploys(ctx context.Context, req types.DeployReq) ([]types.DeployRes, int, error)
	GetClusterByID(ctx context.Context, clusterId string) (*database.ClusterInfo, error)
	GetClusterNodes(ctx context.Context) ([]database.ClusterNodeWithRegion, error)
	GetClusterNodeByID(ctx context.Context, id int64) (*database.ClusterNodeWithRegion, error)
	QueryClusterDeploys(ctx context.Context, req types.ClusterDeployReq) ([]database.Deploy, int, error)
	QueryClusterWorkflows(ctx context.Context, req types.ClusterWFReq) ([]database.ArgoWorkflow, int, error)
	UpdateClusterNodeVXPU(ctx context.Context, req types.UpdateClusterNodeReq) (*database.ClusterNodeWithRegion, error)
	SetClusterNodeAccessMode(ctx context.Context, req types.SetNodeAccessModeReq) error
}

func NewClusterComponent(config *config.Config) (ClusterComponent, error) {
	c := &clusterComponentImpl{}
	c.config = config
	c.deployer = deploy.NewDeployer()
	c.clusterStore = database.NewClusterInfoStore()
	c.deployTaskStore = database.NewDeployTaskStore()
	acctClient, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	c.acctClient = acctClient
	usrClient := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.usrClient = usrClient
	c.resStore = database.NewSpaceResourceStore()
	c.workflowStore = database.NewArgoWorkFlowStore()
	return c, nil
}

type clusterComponentImpl struct {
	deployer        deploy.Deployer
	clusterStore    database.ClusterInfoStore
	deployTaskStore database.DeployTaskStore
	acctClient      accounting.AccountingClient
	config          *config.Config
	resStore        database.SpaceResourceStore
	workflowStore   database.ArgoWorkFlowStore
	usrClient       rpc.UserSvcClient
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

func (c *clusterComponentImpl) IndexPublic(ctx context.Context) (types.PublicClusterRes, error) {
	clusterInos, err := c.clusterStore.List(ctx)
	if err != nil {
		return types.PublicClusterRes{}, err
	}
	var publicClusters types.PublicClusterRes
	gpuVendorMap := make(map[string]bool)
	for _, clusterInfo := range clusterInos {
		if types.ClusterStatus(clusterInfo.Status) == types.ClusterStatusUnavailable {
			continue
		}
		if !clusterInfo.Enable {
			continue
		}
		// Get cluster details to include GPU information
		clusterRes, err := c.deployer.GetClusterById(ctx, clusterInfo.ClusterID)
		var hardware []types.HardwareInfo
		if err == nil {
			// Use NodeResourceInfo from clusterRes.Resources
			for _, nodeRes := range clusterRes.Resources {
				if nodeRes.XPUModel != "" {
					gpuVendorMap[nodeRes.GPUVendor] = true
					bitIntXPUMem, err := units.ParseBigBytes(nodeRes.XPUMem)
					if err != nil {
						slog.WarnContext(ctx, "parse xpu mem failed", "xpu_mem", nodeRes.XPUMem, "error", err)
						bitIntXPUMem = big.NewInt(0)
					}
					hardware = append(hardware, types.HardwareInfo{
						Region:    clusterInfo.Region,
						GPUVendor: nodeRes.GPUVendor,
						XPUModel:  nodeRes.XPUModel,
						XPUMem:    bitIntXPUMem.Int64() / (1024 * 1024 * 1024),
					})
				}
			}
		}
		publicClusters.Hardware = append(publicClusters.Hardware, hardware...)
		publicClusters.Regions = append(publicClusters.Regions, clusterInfo.Region)
	}
	publicClusters.GPUVendors = make([]string, 0, len(gpuVendorMap))
	for vendor := range gpuVendorMap {
		publicClusters.GPUVendors = append(publicClusters.GPUVendors, vendor)
	}
	return publicClusters, nil
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

func (c *clusterComponentImpl) GetClusterUsages(ctx context.Context) ([]types.ClusterRes, error) {
	clusterList, err := c.clusterStore.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list clusters from db", slog.Any("error", err))
		return nil, err
	}

	if len(clusterList) == 0 {
		return []types.ClusterRes{}, nil
	}

	var result []types.ClusterRes
	for _, cluster := range clusterList {
		if !cluster.Enable {
			continue
		}

		clusterRes, err := c.getClusterUsageById(ctx, cluster.ClusterID)
		if err != nil {
			slog.ErrorContext(ctx, "get cluster usage failed",
				slog.String("clusterID", cluster.ClusterID),
				slog.Any("error", err))

			result = append(result, types.ClusterRes{
				ClusterID: cluster.ClusterID,
				Status:    types.ClusterStatusUnavailable,
				Region:    cluster.Region,
				Zone:      cluster.Zone,
				Provider:  cluster.Provider,
			})
		}

		result = append(result, *clusterRes)
	}

	return result, nil
}

func (d *clusterComponentImpl) getClusterUsageById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	cluster, err := d.clusterStore.GetClusterResources(ctx, clusterId)
	if err != nil {
		return nil, err
	}

	res := types.ClusterRes{
		ClusterID:      cluster.ClusterID,
		Region:         cluster.Region,
		Zone:           cluster.Zone,
		Provider:       cluster.Provider,
		Status:         cluster.Status,
		LastUpdateTime: cluster.LastUpdateTime,
		Enable:         cluster.Enable,
	}

	var vendorSet = make(map[string]struct{}, 0)
	var modelsSet = make(map[string]struct{}, 0)

	offlineNodeNum := 0
	for _, node := range cluster.Resources {
		res.TotalCPU += node.TotalCPU
		res.AvailableCPU += node.AvailableCPU
		res.TotalMem += float64(node.TotalMem)
		res.AvailableMem += float64(node.AvailableMem)
		res.TotalGPU += node.TotalXPU
		res.AvailableGPU += node.AvailableXPU
		res.TotalVXPU += node.TotalVXPU
		res.UsedVXPUNum += node.UsedVXPUNum
		res.TotalVXPUMem += node.TotalVXPUMem
		res.AvailableVXPUMem += node.AvailableVXPUMem
		if node.GPUVendor != "" {
			vendorSet[node.GPUVendor] = struct{}{}
			modelsSet[fmt.Sprintf("%s(%s)", node.XPUModel, node.XPUMem)] = struct{}{}
		}
		if (time.Now().Unix() - node.UpdateAt) > int64(d.config.Runner.HearBeatIntervalInSec*2) {
			offlineNodeNum++
		}
	}

	var vendor string
	for k := range vendorSet {
		vendor += k + ", "
	}
	if vendor != "" {
		vendor = vendor[:len(vendor)-2]
	}

	var models string
	for k := range modelsSet {
		models += k + ", "
	}
	if models != "" {
		models = models[:len(models)-2]
	}

	res.XPUVendors = vendor
	res.XPUModels = models

	res.AvailableCPU = math.Floor(res.AvailableCPU)
	res.TotalMem = math.Floor(res.TotalMem)
	res.AvailableMem = math.Floor(res.AvailableMem)
	res.NodeNumber = len(cluster.Resources)
	res.NodeOfflines = offlineNodeNum

	if res.TotalCPU > 0 {
		res.CPUUsage = math.Round((res.TotalCPU-res.AvailableCPU)/res.TotalCPU*100) / 100
	}
	if res.TotalMem > 0 {
		res.MemUsage = math.Round((res.TotalMem-res.AvailableMem)/res.TotalMem*100) / 100
	}
	if res.TotalGPU > 0 {
		res.GPUUsage = math.Round(float64(res.TotalGPU-res.AvailableGPU)/float64(res.TotalGPU)*100) / 100
	}
	if res.TotalVXPU > 0 {
		res.VXPUUsage = math.Round(float64(res.UsedVXPUNum)/float64(res.TotalVXPU)*100) / 100
	}
	if res.TotalVXPUMem > 0 {
		res.VXPUMemUsage = math.Round(float64(res.TotalVXPUMem-res.AvailableVXPUMem)/float64(res.TotalVXPUMem)*100) / 100
	}

	return &res, err
}
