//go:build !ee && !saas

package component

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func (s *serviceComponentImpl) runServiceMultiHost(ctx context.Context, req types.SVCRequest) error {
	return fmt.Errorf("multi-host inference is not supported")
}

func (s *serviceComponentImpl) RemoveWorkset(ctx context.Context, cluster cluster.Cluster, ksvc *v1.Service) error {
	return nil
}

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
