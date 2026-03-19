package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	rtypes "opencsg.com/csghub-server/runner/types"
	nodeutils "opencsg.com/csghub-server/runner/utils"
)

func GenerateResources(params rtypes.ResourceGeneratorParams) *rtypes.GeneratedResources {
	hardware := params.Hardware
	deployExt := params.DeployExt
	config := params.Config
	nodeSelector := make(map[string]string)
	resReq := make(map[corev1.ResourceName]resource.Quantity)

	// Helper function to process labels
	addLabels := func(labels map[string]string) {
		for key, value := range labels {
			nodeSelector[key] = value
		}
	}

	// Process all hardware labels
	hardwareTypes := []struct {
		labels map[string]string
	}{
		{hardware.Gpu.Labels},
		{hardware.Npu.Labels},
		{hardware.Gcu.Labels},
		{hardware.Mlu.Labels},
		{hardware.Dcu.Labels},
		{hardware.GPGpu.Labels},
		{hardware.Cpu.Labels},
	}

	for _, hw := range hardwareTypes {
		if hw.labels != nil {
			addLabels(hw.labels)
		}
	}

	// Process CPU resources
	if hardware.Cpu.Num != "" {
		qty := parseResource(hardware.Cpu.Num)
		resReq[corev1.ResourceCPU] = qty
	}

	// Process memory resources
	if hardware.Memory != "" {
		qty := parseResource(hardware.Memory)
		resReq[corev1.ResourceMemory] = qty
	}

	// Process ephemeral storage
	if hardware.EphemeralStorage != "" {
		qty := parseResource(hardware.EphemeralStorage)
		resReq[corev1.ResourceEphemeralStorage] = qty
	}

	nodeAffinity := handleAccelerator(hardware, resReq, params.Nodes, config)
	// Merge node affinity
	finalNodeAffinity := nodeutils.MergeNodeAffinity(nodeAffinity, deployExt.NodeAffinity)
	// Convert tolerations
	finalTolerations := nodeutils.ToCoreV1Tolerations(deployExt.Tolerations)

	return &rtypes.GeneratedResources{
		ResourceRequirements: resReq,
		NodeSelector:         nodeSelector,
		NodeAffinity:         finalNodeAffinity,
		Tolerations:          finalTolerations,
	}
}

// Helper function to parse resource quantities
func parseResource(value string) resource.Quantity {
	if value == "" {
		return resource.Quantity{}
	}
	return resource.MustParse(value)
}
