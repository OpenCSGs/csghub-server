package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployer_GetClusterUsageById(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		clusterID := "test_cluster"

		mockClsStore := mockdb.NewMockClusterInfoStore(t)
		mockClsStore.EXPECT().GetClusterResources(mock.Anything, clusterID).Return(&types.ClusterRes{
			ClusterID: clusterID,
			Region:    "test_region",
			Zone:      "test_zone",
			Provider:  "test_provider",
			Status:    types.ClusterStatusRunning,
			Resources: []types.NodeResourceInfo{
				{
					NodeName:   "node1",
					NodeStatus: "ready",
					NodeHardware: types.NodeHardware{
						TotalCPU:     12,
						AvailableCPU: 3,
						TotalMem:     24576,
						AvailableMem: 12288,
						TotalXPU:     3,
						AvailableXPU: 1,
					},
				},
			},
		}, nil)

		d := &deployer{
			clusterStore: mockClsStore,
		}

		res, err := d.GetClusterUsageById(context.TODO(), clusterID)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, clusterID, res.ClusterID)
		require.Equal(t, "test_region", res.Region)
		require.Equal(t, "test_zone", res.Zone)
		require.Equal(t, "test_provider", res.Provider)
		require.Equal(t, types.ClusterStatusRunning, res.Status)
		require.Equal(t, 1, res.NodeNumber)

		require.Equal(t, float64(12), res.TotalCPU)
		require.Equal(t, float64(3), res.AvailableCPU)
		require.Equal(t, float64(24576), res.TotalMem)
		require.Equal(t, float64(12288), res.AvailableMem)
		require.Equal(t, int64(3), res.TotalGPU)
		require.Equal(t, int64(1), res.AvailableGPU)

		require.Equal(t, float64(0.75), res.CPUUsage)
		require.Equal(t, float64(0.5), res.MemUsage)
		require.Equal(t, float64(0.67), res.GPUUsage)
	})

	t.Run("no nodes", func(t *testing.T) {
		clusterID := "test_cluster_no_nodes"
		mockClsStore := mockdb.NewMockClusterInfoStore(t)
		mockClsStore.EXPECT().GetClusterResources(mock.Anything, clusterID).Return(&types.ClusterRes{
			ClusterID: clusterID,
			Resources: []types.NodeResourceInfo{},
		}, nil)

		d := &deployer{
			clusterStore: mockClsStore,
		}

		res, err := d.GetClusterUsageById(context.TODO(), clusterID)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, 0, res.NodeNumber)
		require.Equal(t, float64(0), res.TotalCPU)
		require.Equal(t, float64(0), res.AvailableCPU)
		require.Equal(t, float64(0), res.TotalMem)
		require.Equal(t, float64(0), res.AvailableMem)
		require.Equal(t, int64(0), res.TotalGPU)
		require.Equal(t, int64(0), res.AvailableGPU)

		require.Equal(t, float64(0), res.CPUUsage)
		require.Equal(t, float64(0), res.MemUsage)
		require.Equal(t, float64(0), res.GPUUsage)
	})

	t.Run("image runner error", func(t *testing.T) {
		clusterID := "test_cluster_error"
		expectedErr := errors.New("image runner error")
		mockClsStore := mockdb.NewMockClusterInfoStore(t)
		mockClsStore.EXPECT().GetClusterResources(mock.Anything, clusterID).Return(nil, expectedErr)

		d := &deployer{
			clusterStore: mockClsStore,
		}

		res, err := d.GetClusterUsageById(context.TODO(), clusterID)
		require.Error(t, err)
		require.Equal(t, expectedErr, err)
		require.Nil(t, res)
	})
}
