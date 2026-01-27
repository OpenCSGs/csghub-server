package cluster

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// GetResourceInNamespace retrieves namespace resource quota.
func (cluster *Cluster) GetResourceInNamespace(namespace string, quotaName string, config *config.Config) (map[string]types.NodeResourceInfo, error) {
	clientset := cluster.Client
	quota, err := clientset.CoreV1().ResourceQuotas(namespace).Get(context.TODO(), quotaName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	hard := quota.Status.Hard
	used := quota.Status.Used

	// Helper function to calculate available resources for a given resource name.
	calculateAvailable := func(resourceName v1.ResourceName) resource.Quantity {
		hardLimit, ok := hard[common.QuotaRequest+resourceName]
		if !ok {
			// If no hard limit is set, available is zero.
			return *resource.NewQuantity(0, resource.DecimalSI)
		}
		usedAmount, ok := used[common.QuotaRequest+resourceName]
		if !ok {
			// If not used, available is the hard limit.
			return hardLimit.DeepCopy()
		}

		available := hardLimit.DeepCopy()
		available.Sub(usedAmount)
		return available
	}
	xpuCapacityLabel, xpuTypeLabel, _ := getXPULabel(quota.Labels, config)
	gpuModelVendor, gpuModel := getGpuTypeAndVendor(quota.Labels[xpuTypeLabel], xpuCapacityLabel)
	var totalXPU int64 = 0
	var availableXPU int64 = 0
	resourceName := v1.ResourceName(xpuCapacityLabel)
	if hardLimit, ok := hard[common.QuotaRequest+resourceName]; ok {
		totalXPU = parseQuantityToInt64(hardLimit)
		availableXPU = parseQuantityToInt64(calculateAvailable(resourceName))
	}

	totalCPU := resource.Quantity{}
	availableCPU := resource.Quantity{}
	if hardCPU, ok := hard[common.QuotaRequest+v1.ResourceCPU]; ok {
		totalCPU = hardCPU
		availableCPU = calculateAvailable(v1.ResourceCPU)
	}

	totalMem := resource.Quantity{}
	availableMem := resource.Quantity{}
	if hardMem, ok := hard[common.QuotaRequest+v1.ResourceMemory]; ok {
		totalMem = hardMem
		availableMem = calculateAvailable(v1.ResourceMemory)
	}

	return map[string]types.NodeResourceInfo{
		"": {
			NodeHardware: types.NodeHardware{
				TotalCPU:         millicoresToCores(totalCPU.MilliValue()),
				AvailableCPU:     millicoresToCores(availableCPU.MilliValue()),
				TotalMem:         getMem(totalMem.Value()),
				AvailableMem:     getMem(availableMem.Value()),
				TotalXPU:         totalXPU,
				AvailableXPU:     availableXPU,
				XPUCapacityLabel: xpuCapacityLabel,
				XPUModel:         gpuModel,
				GPUVendor:        gpuModelVendor,
			},
		},
	}, nil
}
