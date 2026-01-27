package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func (d *deployer) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	clusterRes, err := d.clusterStore.GetClusterResources(ctx, clusterId)
	if err != nil {
		slog.Warn("failed to get cluster by id in deployer", slog.Any("cluster_id", clusterId), slog.Any("error", err))
		return &types.ClusterRes{
			ClusterID: clusterId,
			Status:    types.ClusterStatusUnavailable,
		}, nil
	}

	resources, err := d.calcResources(ctx, clusterId, clusterRes)
	if err != nil {
		return nil, err
	}

	result := types.ClusterRes{
		ClusterID:      clusterRes.ClusterID,
		Region:         clusterRes.Region,
		Zone:           clusterRes.Zone,
		Provider:       clusterRes.Provider,
		Resources:      resources,
		ResourceStatus: clusterRes.ResourceStatus,
		Status:         types.ClusterStatusRunning,
		NodeNumber:     len(resources),
	}
	for _, node := range result.Resources {
		result.AvailableCPU += node.AvailableCPU
		result.AvailableGPU += node.AvailableXPU
		result.AvailableMem += float64(node.AvailableMem)
		result.TotalCPU += node.TotalCPU
		result.TotalMem += float64(node.TotalMem)
		result.TotalGPU += node.TotalXPU
	}
	result.CPUUsage = (result.TotalCPU - result.AvailableCPU) / result.TotalCPU
	result.MemUsage = (result.TotalMem - result.AvailableMem) / result.TotalMem
	result.GPUUsage = float64(result.TotalGPU-result.AvailableGPU) / float64(result.TotalGPU)
	return &result, err
}

func (d *deployer) GetClusterUsageById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	resp, err := d.imageRunner.GetClusterById(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	res := types.ClusterRes{
		ClusterID: resp.ClusterID,
		Region:    resp.Region,
		Zone:      resp.Zone,
		Provider:  resp.Provider,
		Status:    types.ClusterStatusRunning,
	}
	var vendorSet = make(map[string]struct{}, 0)
	var modelsSet = make(map[string]struct{}, 0)
	for _, node := range resp.Resources {
		res.TotalCPU += node.TotalCPU
		res.AvailableCPU += node.AvailableCPU
		res.TotalMem += float64(node.TotalMem)
		res.AvailableMem += float64(node.AvailableMem)
		res.TotalGPU += node.TotalXPU
		res.AvailableGPU += node.AvailableXPU
		if node.GPUVendor != "" {
			vendorSet[node.GPUVendor] = struct{}{}
			modelsSet[fmt.Sprintf("%s(%s)", node.XPUModel, node.XPUMem)] = struct{}{}
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
	res.NodeNumber = len(resp.Resources)
	if res.TotalCPU != 0 {
		res.CPUUsage = math.Round((res.TotalCPU-res.AvailableCPU)/res.TotalCPU*100) / 100
	}
	if res.TotalMem != 0 {
		res.MemUsage = math.Round((res.TotalMem-res.AvailableMem)/res.TotalMem*100) / 100
	}
	if res.TotalGPU != 0 {
		res.GPUUsage = math.Round(float64(res.TotalGPU-res.AvailableGPU)/float64(res.TotalGPU)*100) / 100
	}

	return &res, err
}

func (d *deployer) CheckResourceAvailable(ctx context.Context, clusterId string, orderDetailID int64, hardWare *types.HardWare) (bool, error) {
	// backward compatibility for old api
	if clusterId == "" {
		clusters, err := d.ListCluster(ctx)
		if err != nil {
			return false, err
		}
		if len(clusters) == 0 {
			return false, fmt.Errorf("can not list clusters")
		}
		clusterId = clusters[0].ClusterID
	}
	clusterResources, err := d.GetClusterById(ctx, clusterId)
	if err != nil {
		return false, err
	}
	err = d.checkOrderDetailByID(ctx, orderDetailID)
	if err != nil {
		return false, err
	}

	if clusterResources.Status == types.ClusterStatusUnavailable {
		err := fmt.Errorf("failed to check cluster available resource due to cluster %s status is %s",
			clusterId, clusterResources.Status)
		return false, errorx.ClusterUnavailable(err, errorx.Ctx().
			Set("cluster ID", clusterId).
			Set("region", clusterResources.Region))
	}

	if clusterResources.ResourceStatus != types.StatusUncertain && !CheckResource(clusterResources, hardWare) {
		err := fmt.Errorf("required resource on cluster %s is not enough with resource status %s",
			clusterId, clusterResources.ResourceStatus)
		return false, errorx.NotEnoughResource(err, errorx.Ctx().
			Set("cluster ID", clusterId).
			Set("region", clusterResources.Region))
	}

	return true, nil
}

func CheckResource(clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	if hardware == nil {
		slog.Error("hardware is empty for check resource", slog.Any("clusterResources", clusterResources))
		return false
	}
	mem, err := strconv.Atoi(strings.ReplaceAll(hardware.Memory, "Gi", ""))
	if err != nil {
		slog.Error("failed to parse hardware memory for check resource", slog.Any("error", err))
		return false
	}
	if hardware.Replicas > 1 {
		return checkMultiNodeResource(mem, clusterResources, hardware)
	} else {
		return checkSingleNodeResource(mem, clusterResources, hardware)
	}
}

// check resource for sigle node
func checkSingleNodeResource(mem int, clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	for _, node := range clusterResources.Resources {
		if float32(mem) <= node.AvailableMem {
			isAvailable := checkNodeResource(node, hardware)
			if isAvailable {
				// if true return, otherwise continue check next node
				return true
			}
		}
	}
	return false
}

func checkMultiNodeResource(mem int, clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	ready := 0
	for _, node := range clusterResources.Resources {
		if float32(mem) <= node.AvailableMem {
			isAvailable := checkNodeResource(node, hardware)
			if isAvailable {
				ready++
				if ready >= hardware.Replicas {
					return true
				}
			}
		}
	}
	return false
}
