//go:build !ee && !saas

package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func handleAccelerator(hardware types.HardWare, resReq map[corev1.ResourceName]resource.Quantity, nodes []types.Node, config *config.Config) *corev1.NodeAffinity {
	// Process accelerator resources
	accelerators := []struct {
		resourceName string
		num          string
	}{
		{hardware.Gpu.ResourceName, hardware.Gpu.Num},
		{hardware.Npu.ResourceName, hardware.Npu.Num},
		{hardware.Gcu.ResourceName, hardware.Gcu.Num},
		{hardware.Mlu.ResourceName, hardware.Mlu.Num},
		{hardware.Dcu.ResourceName, hardware.Dcu.Num},
		{hardware.GPGpu.ResourceName, hardware.GPGpu.Num},
	}

	for _, acc := range accelerators {
		if len(acc.num) < 1 || len(acc.resourceName) < 1 {
			// skip if num or resource name is empty
			continue
		}
		qty := parseResource(acc.num)
		resReq[corev1.ResourceName(acc.resourceName)] = qty
	}
	return nil
}
