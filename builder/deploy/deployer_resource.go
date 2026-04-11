package deploy

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
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
		result.TotalCPU += node.TotalCPU                  // cpu
		result.AvailableCPU += node.AvailableCPU          // available cpu
		result.TotalMem += float64(node.TotalMem)         // mem
		result.AvailableMem += float64(node.AvailableMem) // available mem
		result.TotalGPU += node.TotalXPU                  // xpu number
		result.AvailableGPU += node.AvailableXPU          // available xpu number

		result.TotalVXPU += node.TotalVXPU               // total vxpu number
		result.UsedVXPUNum += node.UsedVXPUNum           // used vxpu number
		result.TotalVXPUMem += node.TotalVXPUMem         // total vxpu mem in MB
		result.AvailableVXPUMem += node.AvailableVXPUMem // available vxpu mem in MB
	}
	if result.TotalCPU > 0 {
		result.CPUUsage = (result.TotalCPU - result.AvailableCPU) / result.TotalCPU
	}
	if result.TotalMem > 0 {
		result.MemUsage = (result.TotalMem - result.AvailableMem) / result.TotalMem
	}
	if result.TotalGPU > 0 {
		result.GPUUsage = float64(result.TotalGPU-result.AvailableGPU) / float64(result.TotalGPU)
	}
	if result.TotalVXPU > 0 {
		result.VXPUUsage = float64(result.UsedVXPUNum) / float64(result.TotalVXPU)
	}
	if result.TotalVXPUMem > 0 {
		result.VXPUMemUsage = float64(result.TotalVXPUMem-result.AvailableVXPUMem) / float64(result.TotalVXPUMem)
	}
	return &result, err
}

func (d *deployer) CheckResourceAvailable(ctx context.Context, clusterId string, orderDetailID int64, hardWare *types.HardWare) (bool, []types.ResourceAvailableStatus, error) {
	// backward compatibility for old api
	if clusterId == "" {
		clusters, err := d.ListCluster(ctx)
		if err != nil {
			return false, nil, err
		}
		if len(clusters) == 0 {
			return false, nil, fmt.Errorf("can not list clusters")
		}
		clusterId = clusters[0].ClusterID
	}
	clusterResources, err := d.GetClusterById(ctx, clusterId)
	if err != nil {
		return false, nil, err
	}
	err = d.checkOrderDetailByID(ctx, orderDetailID)
	if err != nil {
		return false, nil, err
	}

	if clusterResources.Status == types.ClusterStatusUnavailable {
		err := fmt.Errorf("failed to check cluster available resource due to cluster %s status is %s",
			clusterId, clusterResources.Status)
		return false, nil, errorx.ClusterUnavailable(err, errorx.Ctx().
			Set("cluster ID", clusterId).
			Set("region", clusterResources.Region))
	}

	available, availableStatusList := CheckResource(clusterResources, hardWare, d.config)
	if d.IsDefaultScheduler() &&
		clusterResources.ResourceStatus != types.StatusUncertain &&
		!available {
		err := fmt.Errorf("required resource on cluster %s is not enough with resource status %s",
			clusterId, clusterResources.ResourceStatus)
		return false, availableStatusList, errorx.NotEnoughResource(err, errorx.Ctx().
			Set("cluster ID", clusterId).
			Set("region", clusterResources.Region))
	}

	return true, availableStatusList, nil
}

func CheckResource(clusterResources *types.ClusterRes, hardware *types.HardWare, config *config.Config) (bool, []types.ResourceAvailableStatus) {
	if hardware == nil {
		slog.Error("hardware is empty for check resource", slog.Any("clusterResources", clusterResources))
		return false, []types.ResourceAvailableStatus{
			{
				Available: false,
				Reason:    types.UnAvailableTypeInvalidHardware,
			},
		}
	}
	if hardware.Replicas > 1 {
		return checkMultiNodeResource(clusterResources, hardware, config)
	} else {
		return checkSingleNodeResource(clusterResources, hardware, config)
	}
}

// check resource for sigle node
func checkSingleNodeResource(clusterResources *types.ClusterRes, hardware *types.HardWare, config *config.Config) (bool, []types.ResourceAvailableStatus) {
	var availableStatusList []types.ResourceAvailableStatus
	for _, node := range clusterResources.Resources {
		availableStatus := checkNodeResource(node, hardware, config)
		availableStatusList = append(availableStatusList, availableStatus)
		if availableStatus.Available {
			// if true return, otherwise continue check next node
			return true, availableStatusList
		}
	}
	return false, availableStatusList
}

func checkMultiNodeResource(clusterResources *types.ClusterRes, hardware *types.HardWare, config *config.Config) (bool, []types.ResourceAvailableStatus) {
	var availableStatusList []types.ResourceAvailableStatus
	ready := 0
	for _, node := range clusterResources.Resources {
		availableStatus := checkNodeResource(node, hardware, config)
		availableStatusList = append(availableStatusList, availableStatus)
		if availableStatus.Available {
			ready++
			if ready >= hardware.Replicas {
				return true, availableStatusList
			}
		}
	}
	return false, availableStatusList
}

func isCPUOnlyWorkload(hardware *types.HardWare) bool {
	if hardware == nil {
		return false
	}

	return hardware.Gpu.Num == "" &&
		hardware.Npu.Num == "" &&
		hardware.Gcu.Num == "" &&
		hardware.Mlu.Num == "" &&
		hardware.Dcu.Num == "" &&
		hardware.GPGpu.Num == ""
}

func isXPUNode(node types.NodeResourceInfo) bool {
	return node.HasXPU()
}
