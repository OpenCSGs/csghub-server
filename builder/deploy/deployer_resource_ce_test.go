//go:build !ee && !saas

package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestCheckResource(t *testing.T) {
	cfg := &config.Config{}

	testCases := []struct {
		name             string
		clusterResources *types.ClusterRes
		hardware         *types.HardWare
		want             bool
	}{
		{
			name: "nil hardware should return false",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
				},
			},
			hardware: nil,
			want:     false,
		},
		{
			name: "single node - resource sufficient",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
							VXPUs: []types.VXPU{
								{Mem: 24000, AllocatedMem: 0},
								{Mem: 24000, AllocatedMem: 0},
							},
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 1,
			},
			want: true,
		},
		{
			name: "single node - memory insufficient",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 8,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Replicas: 1,
			},
			want: false,
		},
		{
			name: "single node - invalid memory format",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "invalid-memory",
				Replicas: 1,
			},
			want: false,
		},
		{
			name: "single node - memory unit Mi",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 1, // 1GiB
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "1024Mi",
				Replicas: 1,
			},
			want: true,
		},
		{
			name: "single node - memory boundary exact match",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 10,
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Replicas: 1,
			},
			want: true,
		},
		{
			name: "multi node - enough nodes count but insufficient resources",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
						},
					},
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 4, // Insufficient for 10Gi
						},
					},
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 4, // Insufficient for 10Gi
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Replicas: 2,
			},
			want: false,
		},
		{
			name: "single node - cpu only workload should be rejected on xpu node",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeName: "gpu-node-1",
						NodeHardware: types.NodeHardware{
							AvailableMem:     100,
							AvailableCPU:     16,
							AvailableXPU:     2,
							TotalXPU:         2,
							XPUModel:         "NVIDIA-A100",
							XPUCapacityLabel: "nvidia.com/gpu",
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Replicas: 1,
			},
			want: false,
		},
		{
			name: "single node - cpu only workload should pass on cpu-only node",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeName: "cpu-node-1",
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Replicas: 1,
			},
			want: true,
		},
		{
			name: "multi node - resource sufficient",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
							VXPUs: []types.VXPU{
								{Mem: 24000, AllocatedMem: 0},
								{Mem: 24000, AllocatedMem: 0},
							},
						},
					},
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
							VXPUs: []types.VXPU{
								{Mem: 24000, AllocatedMem: 0},
								{Mem: 24000, AllocatedMem: 0},
							},
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			want: true,
		},
		{
			name: "multi node - not enough nodes",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			want: false,
		},
		{
			name: "multi node - mixed available nodes",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 4,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			want: true,
		},
		{
			name: "single node - no resources in cluster",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Replicas: 1,
			},
			want: false,
		},
		{
			name: "single node - wrong GPU type",
			clusterResources: &types.ClusterRes{
				ClusterID: "c1",
				Resources: []types.NodeResourceInfo{
					{
						NodeHardware: types.NodeHardware{
							AvailableMem: 100,
							AvailableCPU: 16,
							AvailableXPU: 2,
							XPUModel:     "NVIDIA-A100",
						},
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-V100"},
				Replicas: 1,
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := CheckResource(tc.clusterResources, tc.hardware, cfg)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCheckResourceAvailable(t *testing.T) {
	t.Run("success - with cluster ID", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
			ClusterID:      "c1",
			Status:         types.ClusterStatusRunning,
			ResourceStatus: types.StatusClusterWide,
			Resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
		}, nil)

		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Cpu:      types.CPU{Num: "8"},
			Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
			Replicas: 1,
		})

		require.NoError(t, err)
		require.True(t, available)
	})

	t.Run("success - multi-node resource check", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
			ClusterID:      "c1",
			Status:         types.ClusterStatusRunning,
			ResourceStatus: types.StatusClusterWide,
			Resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
		}, nil)

		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Cpu:      types.CPU{Num: "8"},
			Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
			Replicas: 2,
		})

		require.NoError(t, err)
		require.True(t, available)
	})

	t.Run("cluster unavailable", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
			ClusterID: "c1",
			Status:    types.ClusterStatusUnavailable,
			Region:    "us-west-1",
		}, nil)

		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Replicas: 1,
		})

		require.Error(t, err)
		require.False(t, available)
	})

	t.Run("resource not enough", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
			ClusterID:      "c1",
			Status:         types.ClusterStatusRunning,
			ResourceStatus: types.StatusClusterWide,
			Region:         "us-west-1",
			Resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 4,
					},
				},
			},
		}, nil)

		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Replicas: 1,
		})

		require.Error(t, err)
		require.False(t, available)
	})

	t.Run("get cluster by id error", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(nil, errors.New("db error"))

		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Replicas: 1,
		})

		require.Error(t, err)
		require.False(t, available)
	})

	t.Run("uncertain resource status - should pass check", func(t *testing.T) {
		mockStores := tests.NewMockStores(t)
		d := &deployer{
			clusterStore: mockStores.ClusterInfo,
			config:       &config.Config{},
		}
		ctx := context.TODO()

		mockStores.ClusterInfoMock().EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
			ClusterID:      "c1",
			Status:         types.ClusterStatusRunning,
			ResourceStatus: types.StatusUncertain,
			Resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 4,
					},
				},
			},
		}, nil)

		// When ResourceStatus is StatusUncertain, CheckResource is not called and it returns true
		available, _, err := d.CheckResourceAvailable(ctx, "c1", 0, &types.HardWare{
			Memory:   "10Gi",
			Replicas: 1,
		})

		require.NoError(t, err)
		require.True(t, available)
	})
}

func TestCheckSingleNodeResource(t *testing.T) {
	cfg := &config.Config{}

	testCases := []struct {
		name           string
		resources      []types.NodeResourceInfo
		hardware       *types.HardWare
		expectedResult bool
	}{
		{
			name: "single node matches requirements",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 1,
			},
			expectedResult: true,
		},
		{
			name: "no node has enough memory",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 4,
						AvailableCPU: 16,
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 6,
						AvailableCPU: 16,
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Replicas: 1,
			},
			expectedResult: false,
		},
		{
			name: "second node satisfies requirements",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 4,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 1,
			},
			expectedResult: true,
		},
		{
			name: "no node has enough memory in Mi",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 0.5,
						AvailableCPU: 16,
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "600Mi",
				Replicas: 1,
			},
			expectedResult: false,
		},
		{
			name: "satisfies requirements in Mi",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 0.5,
						AvailableCPU: 16,
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "200Mi",
				Replicas: 1,
			},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusterRes := &types.ClusterRes{
				ClusterID: "c1",
				Resources: tc.resources,
			}
			result, _ := checkSingleNodeResource(clusterRes, tc.hardware, cfg)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestCheckMultiNodeResource(t *testing.T) {
	cfg := &config.Config{}

	testCases := []struct {
		name           string
		resources      []types.NodeResourceInfo
		hardware       *types.HardWare
		expectedResult bool
	}{
		{
			name: "enough nodes for multi-node deployment",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			expectedResult: true,
		},
		{
			name: "not enough nodes for multi-node deployment",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			expectedResult: false,
		},
		{
			name: "exact number of nodes",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			expectedResult: true,
		},
		{
			name: "one node does not satisfy GPU requirement",
			resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-A100",
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableMem: 100,
						AvailableCPU: 16,
						AvailableXPU: 2,
						XPUModel:     "NVIDIA-V100", // Different GPU type
					},
				},
			},
			hardware: &types.HardWare{
				Memory:   "10Gi",
				Cpu:      types.CPU{Num: "8"},
				Gpu:      types.Processor{Num: "1", Type: "NVIDIA-A100"},
				Replicas: 2,
			},
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusterRes := &types.ClusterRes{
				ClusterID: "c1",
				Resources: tc.resources,
			}
			result, _ := checkMultiNodeResource(clusterRes, tc.hardware, cfg)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}
