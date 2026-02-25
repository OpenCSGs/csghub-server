//go:build !ee && !saas

package cluster

import (
	v1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/common/types"
)

func collectNodeVXPU(node v1.Node) []types.VXPU {
	return []types.VXPU{}
}

func collectPodVXPU(pod v1.Pod) []types.VXPU {
	return []types.VXPU{}
}

func calcSingleNodeXPUMem(nodeRes *types.NodeResourceInfo) *types.NodeResourceInfo {
	return nodeRes
}

// collect mig resource
func collectMIGResources(node v1.Node) map[string]*types.MIGResource {
	migResources := make(map[string]*types.MIGResource)
	return migResources
}

func calcMIGResource(resources v1.ResourceRequirements, migs map[string]*types.MIGResource) {
}
