//go:build ee || saas

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func Test_handleAccelerator(t *testing.T) {
	config := &config.Config{}
	config.Runner.VGPUResourceReqKey = "nvidia.com/vgpu"
	config.Runner.VGPUMemoryReqKey = "nvidia.com/vgpumem"

	tests := []struct {
		name     string
		hardware types.HardWare
		nodes    []types.Node
		expected struct {
			hasResources bool
			resReqKeys   []corev1.ResourceName
			affinityType string
			nodeNames    []string
		}
	}{
		{
			name: "No XPU request - schedule on CPU-only nodes when available",
			hardware: types.HardWare{
				Cpu:    types.CPU{Num: "2"},
				Memory: "4Gi",
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true, HasXPU: true},
				{Name: "node-2", EnableVXPU: false},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: false,
				resReqKeys:   []corev1.ResourceName{},
				affinityType: "cpu",
				nodeNames:    []string{"node-1"},
			},
		},
		{
			name: "No XPU request - forbid scheduling on all-XPU clusters",
			hardware: types.HardWare{
				Cpu:    types.CPU{Num: "2"},
				Memory: "4Gi",
			},
			nodes: []types.Node{
				{Name: "node-1", HasXPU: true},
				{Name: "node-2", EnableVXPU: true, HasXPU: true},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: false,
				resReqKeys:   []corev1.ResourceName{},
				affinityType: "cpu",
				nodeNames:    []string{"node-1", "node-2"},
			},
		},
		{
			name: "Physical GPU request with mixed nodes - schedule on non-vxpu nodes",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
				{Name: "node-2", EnableVXPU: false},
				{Name: "node-3", EnableVXPU: false},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{"nvidia.com/gpu"},
				affinityType: "physical",
				nodeNames:    []string{"node-2", "node-3"},
			},
		},
		{
			name: "vGPU request with mixed nodes - schedule on vxpu nodes",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:             "2",
					ResourceName:    config.Runner.VGPUResourceReqKey,
					ResourceMemName: config.Runner.VGPUMemoryReqKey,
					MemSize:         "4096",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
				{Name: "node-2", EnableVXPU: true},
				{Name: "node-3", EnableVXPU: false},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{corev1.ResourceName(config.Runner.VGPUResourceReqKey), corev1.ResourceName(config.Runner.VGPUMemoryReqKey)},
				affinityType: "vxpu",
				nodeNames:    []string{"node-1", "node-2"},
			},
		},
		{
			name: "Physical GPU request with only vxpu nodes - avoid vxpu nodes",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
				{Name: "node-2", EnableVXPU: true},
				{Name: "node-3", EnableVXPU: true},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{"nvidia.com/gpu"},
				affinityType: "physical",
				nodeNames:    []string{"node-1", "node-2", "node-3"},
			},
		},
		{
			name: "vGPU request with only non-vxpu nodes - avoid non-vxpu nodes",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:             "1",
					ResourceName:    config.Runner.VGPUResourceReqKey,
					ResourceMemName: config.Runner.VGPUMemoryReqKey,
					MemSize:         "8192",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: false},
				{Name: "node-2", EnableVXPU: false},
				{Name: "node-3", EnableVXPU: false},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{corev1.ResourceName(config.Runner.VGPUResourceReqKey), corev1.ResourceName(config.Runner.VGPUMemoryReqKey)},
				affinityType: "vxpu",
				nodeNames:    []string{"node-1", "node-2", "node-3"},
			},
		},
		{
			name: "Multiple accelerator types - NPU and GPU",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
				Npu: types.Processor{
					Num:          "2",
					ResourceName: "huawei.com/npu",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: false},
				{Name: "node-2", EnableVXPU: true},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{"nvidia.com/gpu", "huawei.com/npu"},
				affinityType: "physical",
				nodeNames:    []string{"node-1"},
			},
		},
		{
			name: "Empty nodes list - returns nil affinity without processing resources",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
			},
			nodes: []types.Node{},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys:   []corev1.ResourceName{"nvidia.com/gpu"},
				affinityType: "none",
				nodeNames:    []string{},
			},
		},
		{
			name: "GPU with empty resource name - treated as no XPU request",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: false,
				resReqKeys:   []corev1.ResourceName{},
				affinityType: "cpu",
				nodeNames:    []string{"node-1"},
			},
		},
		{
			name: "GPU with empty num - treated as no XPU request",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "",
					ResourceName: "nvidia.com/gpu",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: false,
				resReqKeys:   []corev1.ResourceName{},
				affinityType: "cpu",
				nodeNames:    []string{"node-1"},
			},
		},
		{
			name: "All accelerator types with vGPU",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:             "2",
					ResourceName:    config.Runner.VGPUResourceReqKey,
					ResourceMemName: config.Runner.VGPUMemoryReqKey,
					MemSize:         "4096",
				},
				Npu: types.Processor{
					Num:          "1",
					ResourceName: "huawei.com/npu",
				},
				Gcu: types.Processor{
					Num:          "1",
					ResourceName: "enflame.com/gcu",
				},
				Mlu: types.Processor{
					Num:          "1",
					ResourceName: "cambricon.com/mlu",
				},
				Dcu: types.Processor{
					Num:          "1",
					ResourceName: "hygon.com/dcu",
				},
				GPGpu: types.Processor{
					Num:          "1",
					ResourceName: "iluvatar.com/gpgpu",
				},
			},
			nodes: []types.Node{
				{Name: "node-1", EnableVXPU: true},
				{Name: "node-2", EnableVXPU: false},
			},
			expected: struct {
				hasResources bool
				resReqKeys   []corev1.ResourceName
				affinityType string
				nodeNames    []string
			}{
				hasResources: true,
				resReqKeys: []corev1.ResourceName{
					corev1.ResourceName(config.Runner.VGPUResourceReqKey),
					corev1.ResourceName(config.Runner.VGPUMemoryReqKey),
					"huawei.com/npu",
					"enflame.com/gcu",
					"cambricon.com/mlu",
					"hygon.com/dcu",
					"iluvatar.com/gpgpu",
				},
				affinityType: "vxpu",
				nodeNames:    []string{"node-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resReq := make(map[corev1.ResourceName]resource.Quantity)

			nodeAffinity := handleAccelerator(tt.hardware, resReq, tt.nodes, config)

			if tt.expected.hasResources {
				require.NotEmpty(t, resReq, "Expected resource requests but got none")
				require.Len(t, resReq, len(tt.expected.resReqKeys), "Unexpected number of resource requests")

				for _, key := range tt.expected.resReqKeys {
					_, exists := resReq[key]
					require.True(t, exists, "Expected resource key %s not found", key)
				}
			} else {
				require.Empty(t, resReq, "Expected no resource requests but got some")
			}

			if tt.expected.affinityType == "none" {
				require.Nil(t, nodeAffinity, "Expected nil node affinity")
				return
			}

			require.NotNil(t, nodeAffinity, "Expected node affinity but got nil")
			require.NotNil(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution, "Expected RequiredDuringSchedulingIgnoredDuringExecution")
			require.Len(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 1, "Expected exactly one NodeSelectorTerm")

			term := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
			require.Len(t, term.MatchExpressions, 1, "Expected exactly one MatchExpression")

			expr := term.MatchExpressions[0]
			require.Equal(t, types.KubernetesNodeName, expr.Key, "Unexpected node selector key")

			if tt.expected.affinityType == "vxpu" {
				hasEnabledVxpuNodes := false
				for _, node := range tt.nodes {
					if node.EnableVXPU {
						hasEnabledVxpuNodes = true
						break
					}
				}

				if hasEnabledVxpuNodes {
					require.Equal(t, corev1.NodeSelectorOpIn, expr.Operator, "Expected OpIn operator for vxpu affinity")
					var enabledNodes []string
					for _, node := range tt.nodes {
						if node.EnableVXPU {
							enabledNodes = append(enabledNodes, node.Name)
						}
					}
					require.ElementsMatch(t, enabledNodes, expr.Values, "Node names don't match for vxpu affinity (OpIn)")
				} else {
					require.Equal(t, corev1.NodeSelectorOpNotIn, expr.Operator, "Expected OpNotIn operator for vxpu affinity")
					var disabledNodes []string
					for _, node := range tt.nodes {
						if !node.EnableVXPU {
							disabledNodes = append(disabledNodes, node.Name)
						}
					}
					require.ElementsMatch(t, disabledNodes, expr.Values, "Node names don't match for vxpu affinity (OpNotIn)")
				}
			} else if tt.expected.affinityType == "physical" {
				hasDisabledVxpuNodes := false
				for _, node := range tt.nodes {
					if !node.EnableVXPU {
						hasDisabledVxpuNodes = true
						break
					}
				}

				if hasDisabledVxpuNodes {
					require.Equal(t, corev1.NodeSelectorOpIn, expr.Operator, "Expected OpIn operator for physical affinity")
					var disabledNodes []string
					for _, node := range tt.nodes {
						if !node.EnableVXPU {
							disabledNodes = append(disabledNodes, node.Name)
						}
					}
					require.ElementsMatch(t, disabledNodes, expr.Values, "Node names don't match for physical affinity (OpIn)")
				} else {
					require.Equal(t, corev1.NodeSelectorOpNotIn, expr.Operator, "Expected OpNotIn operator for physical affinity")
					var enabledNodes []string
					for _, node := range tt.nodes {
						if node.EnableVXPU {
							enabledNodes = append(enabledNodes, node.Name)
						}
					}
					require.ElementsMatch(t, enabledNodes, expr.Values, "Node names don't match for physical affinity (OpNotIn)")
				}
			} else if tt.expected.affinityType == "cpu" {
				require.Equal(t, corev1.NodeSelectorOpNotIn, expr.Operator, "Expected OpNotIn operator for cpu affinity")
				require.ElementsMatch(t, tt.expected.nodeNames, expr.Values, "Node names don't match for cpu affinity")
			}
		})
	}
}
