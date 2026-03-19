//go:build !ee && !saas

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

func Test_handleAccelerator_CE(t *testing.T) {
	tests := []struct {
		name     string
		hardware types.HardWare
		wantReq  map[corev1.ResourceName]resource.Quantity
	}{
		{
			name: "gpu with resource name",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
			},
			wantReq: map[corev1.ResourceName]resource.Quantity{
				"nvidia.com/gpu": resource.MustParse("1"),
			},
		},
		{
			name: "npu with resource name",
			hardware: types.HardWare{
				Npu: types.Processor{
					Num:          "2",
					ResourceName: "huawei.com/npu",
				},
			},
			wantReq: map[corev1.ResourceName]resource.Quantity{
				"huawei.com/npu": resource.MustParse("2"),
			},
		},
		{
			name: "no resource name",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num: "1",
				},
			},
			wantReq: map[corev1.ResourceName]resource.Quantity{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resReq := make(map[corev1.ResourceName]resource.Quantity)
			affinity := handleAccelerator(tt.hardware, resReq, nil, &config.Config{})
			require.Nil(t, affinity)
			require.Equal(t, tt.wantReq, resReq)
		})
	}
}

func Test_GenerateResources_CE(t *testing.T) {
	tests := []struct {
		name      string
		hardware  types.HardWare
		nodes     []types.Node
		deployExt types.DeployExtend
		config    *config.Config
		want      *rtypes.GeneratedResources
	}{
		{
			name: "gpu resources with resource name (CE)",
			hardware: types.HardWare{
				Gpu: types.Processor{
					Num:          "1",
					ResourceName: "nvidia.com/gpu",
				},
			},
			nodes:     []types.Node{},
			deployExt: types.DeployExtend{},
			config:    &config.Config{},
			want: &rtypes.GeneratedResources{
				ResourceRequirements: map[corev1.ResourceName]resource.Quantity{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
				NodeSelector: map[string]string{},
				NodeAffinity: nil,
				Tolerations:  nil,
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
			require.Equal(t, tt.want.NodeAffinity, got.NodeAffinity)
			require.Equal(t, tt.want.Tolerations, got.Tolerations)
		})
	}
}
