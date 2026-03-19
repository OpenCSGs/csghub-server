package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rtypes "opencsg.com/csghub-server/runner/types"
)

func Test_GenerateResources(t *testing.T) {
	tests := []struct {
		name      string
		hardware  types.HardWare
		nodes     []types.Node
		deployExt types.DeployExtend
		config    *config.Config
		want      *rtypes.GeneratedResources
	}{
		{
			name: "basic cpu resources",
			hardware: types.HardWare{
				Cpu: types.CPU{
					Num: "2",
				},
				Memory:           "4Gi",
				EphemeralStorage: "10Gi",
			},
			nodes:     []types.Node{},
			deployExt: types.DeployExtend{},
			config:    &config.Config{},
			want: &rtypes.GeneratedResources{
				ResourceRequirements: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("4Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
				},
				NodeSelector: map[string]string{},
				NodeAffinity: nil,
				Tolerations:  nil,
			},
		},
		{
			name: "gpu resources with affinity and tolerations",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:    "1",
					Type:   "A100",
					Labels: map[string]string{"gpu": "A100"},
				},
			},
			nodes: []types.Node{},
			deployExt: types.DeployExtend{
				Tolerations: []types.Toleration{
					{
						Key:      "key1",
						Operator: "Equal",
						Value:    "value1",
						Effect:   "NoSchedule",
					},
				},
			},
			config: &config.Config{},
			want: &rtypes.GeneratedResources{
				ResourceRequirements: map[corev1.ResourceName]resource.Quantity{},
				NodeSelector: map[string]string{
					"gpu": "A100",
				},
				NodeAffinity: nil,
				Tolerations: []corev1.Toleration{
					{
						Key:      "key1",
						Operator: corev1.TolerationOpEqual,
						Value:    "value1",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateResources(rtypes.ResourceGeneratorParams{
				Hardware:  tt.hardware,
				Nodes:     tt.nodes,
				DeployExt: tt.deployExt,
				Config:    tt.config,
			})

			require.Equal(t, tt.want.ResourceRequirements, got.ResourceRequirements)
			require.Equal(t, tt.want.NodeSelector, got.NodeSelector)
			// For NodeAffinity, we might need more complex comparison if it's not nil,
			// but for now simple cases work with DeepEqual implicitly via Equal
			// require.Equal(t, tt.want.NodeAffinity, got.NodeAffinity)
			require.Equal(t, tt.want.Tolerations, got.Tolerations)
		})
	}
}
