package component

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mockDeploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestClusterComponent_Index(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	cc.mocks.stores.ClusterInfoMock().EXPECT().List(ctx).Return([]database.ClusterInfo{
		{ClusterID: "c1", Status: "running", Enable: true},
		{ClusterID: "c2", Status: "error", Enable: true},
	}, nil)

	data, err := cc.Index(ctx)
	require.Nil(t, err)
	require.Len(t, data, 2)
}

func TestClusterComponent_GetClusterWithResourceByID(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)
	cc.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "c1").Return(database.ClusterInfo{
		ClusterID: "c1",
		Status:    "running",
	}, nil)
	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{
		ClusterID: "c1",
	}, nil)

	data, err := cc.GetClusterWithResourceByID(ctx, "c1")
	require.Nil(t, err)
	require.Equal(t, "c1", data.ClusterID)
}

func TestClusterComponent_Update(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	cc.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "c1").Return(database.ClusterInfo{
		ClusterID: "c1",
		Status:    "running",
	}, nil)

	cc.mocks.stores.ClusterInfoMock().EXPECT().Update(ctx, database.ClusterInfo{
		ClusterID: "c1",
		Status:    "running",
	}).Return(nil)

	data, err := cc.Update(ctx, types.ClusterRequest{ClusterID: "c1"})
	require.Nil(t, err)
	require.Equal(t, "c1", data.ClusterID)
}

func TestClusterComponent_GetClusterUsages(t *testing.T) {
	ctx := context.TODO()

	mockDeployer := mockDeploy.NewMockDeployer(t)
	clsStore := mockdb.NewMockClusterInfoStore(t)

	// Create cluster component with mock
	cfg := &config.Config{}
	cfg.Runner.HearBeatIntervalInSec = 30
	c := &clusterComponentImpl{
		deployer:     mockDeployer,
		clusterStore: clsStore,
		config:       cfg,
	}

	// Test the method
	clsStore.EXPECT().List(ctx).Return([]database.ClusterInfo{
		{ClusterID: "c1", Enable: true, Region: "us-west-1", Zone: "zone-a", Provider: "aws"},
		{ClusterID: "c2", Enable: true, Region: "us-east-1", Zone: "zone-b", Provider: "gcp"},
	}, nil)

	clsStore.EXPECT().GetClusterResources(ctx, "c1").Return(&types.ClusterRes{
		ClusterID: "c1",
		Status:    types.ClusterStatusRunning,
		Region:    "us-west-1",
		Zone:      "zone-a",
		Provider:  "aws",
		Resources: []types.NodeResourceInfo{
			{
				NodeName:   "node-1",
				NodeStatus: "Ready",
				NodeHardware: types.NodeHardware{
					TotalCPU:     64,
					AvailableCPU: 32,
					TotalMem:     256,
					AvailableMem: 128,
					TotalXPU:     8,
					AvailableXPU: 4,
					GPUVendor:    "NVIDIA",
					XPUModel:     "A100",
					XPUMem:       "40Gi",
				},
				UpdateAt: time.Now().Unix(),
			},
		},
	}, nil)

	clsStore.EXPECT().GetClusterResources(ctx, "c2").Return(&types.ClusterRes{
		ClusterID: "c2",
		Status:    types.ClusterStatusRunning,
		Region:    "us-east-1",
		Zone:      "zone-b",
		Provider:  "gcp",
		Resources: []types.NodeResourceInfo{
			{
				NodeName:   "node-2",
				NodeStatus: "Ready",
				NodeHardware: types.NodeHardware{
					TotalCPU:     32,
					AvailableCPU: 16,
					TotalMem:     128,
					AvailableMem: 64,
					TotalXPU:     4,
					AvailableXPU: 2,
					GPUVendor:    "AMD",
					XPUModel:     "MI100",
					XPUMem:       "32Gi",
				},
				UpdateAt: time.Now().Unix(),
			},
		},
	}, nil)

	res, err := c.GetClusterUsages(ctx)

	require.Nil(t, err)
	require.Len(t, res, 2)
	require.Equal(t, "c1", res[0].ClusterID)
	require.Equal(t, "c2", res[1].ClusterID)
}

func TestClusterComponent_GetDeploys(t *testing.T) {
	// Create test data
	testDeploys := []database.Deploy{
		{
			ID:         1,
			ClusterID:  "cluster-1",
			DeployName: "deploy-1",
			Status:     common.Running,
			UserID:     101,
			UserUUID:   "user-uuid-1",
			SvcName:    "service-1",
			Hardware:   "cpu=2,memory=4Gi",
			User: &database.User{
				Username: "testuser1",
				Avatar:   "avatar1.jpg",
			},
		},
	}

	// Create mock for deployTaskStore
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockClusterStore := &mockdb.MockClusterInfoStore{}

	// Create mock for acctClient
	mockAcctClient := accounting.NewMockAccountingClient(t)

	// Create cluster component with mocks
	c := &clusterComponentImpl{
		deployTaskStore: mockDeployStore,
		acctClient:      mockAcctClient,
		clusterStore:    mockClusterStore,
	}

	// Test the method
	result := &types.AcctStatementsResult{
		AcctSummary: types.AcctSummary{
			Total:      200,
			TotalValue: 123.45,
		},
	}
	mockClusterStore.EXPECT().List(context.Background()).Return([]database.ClusterInfo{
		{ClusterID: "cluster-1", Region: "us-west-1"},
	}, nil)
	mockDeployStore.EXPECT().ListDeployByType(context.Background(), types.DeployReq{}).Return(testDeploys, 2, nil)
	mockAcctClient.EXPECT().ListStatementByUserIDAndTime(types.ActStatementsReq{
		Scene:        11,
		UserUUID:     "user-uuid-1",
		StartTime:    testDeploys[0].CreatedAt.Format(time.DateTime),
		EndTime:      time.Now().Format(time.DateTime),
		InstanceName: "service-1",
		Per:          1,
		Page:         1,
	}).Return(result, nil)
	res, total, err := c.GetDeploys(context.Background(), types.DeployReq{})

	// Verify results
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, 2, total)

	// Verify first deploy
	require.Equal(t, "cluster-1", res[0].ClusterID)
	require.Equal(t, "deploy-1", res[0].DeployName)
	require.Equal(t, "Running", res[0].Status)
	require.Equal(t, testDeploys[0].CreatedAt, res[0].CreateTime)
	require.Equal(t, int64(101), res[0].User.ID)
	require.Equal(t, "testuser1", res[0].User.Username)
	require.Equal(t, "avatar1.jpg", res[0].User.Avatar)
	require.Equal(t, "cpu=2,memory=4Gi", res[0].Resource)
	require.Equal(t, 200, res[0].TotalTimeInMin)
	require.Equal(t, 123, res[0].TotalFeeInCents)
	require.Equal(t, "service-1", res[0].SvcName)
}

func TestClusterComponent_GetClusterByID(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)
	cc.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "c1").Return(database.ClusterInfo{
		ClusterID: "c1",
		Status:    "running",
	}, nil)

	data, err := cc.GetClusterByID(ctx, "c1")
	require.Nil(t, err)
	require.Equal(t, "c1", data.ClusterID)
}

func TestClusterComponent_ExtractDeployTargetAndHost1(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)
	cc.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "c1").Return(database.ClusterInfo{
		ClusterID: "c1",
		Status:    "running",
	}, nil)

	req := types.EndpointReq{
		ClusterID: "c1",
		Target:    "t1",
	}

	endpoint, host, err := ExtractDeployTargetAndHost(ctx, cc, req)
	require.Nil(t, err)
	require.Equal(t, "t1", endpoint)
	require.Equal(t, "", host)

}

func TestClusterComponent_ExtractDeployTargetAndHost2(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)
	cc.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "c1").Return(database.ClusterInfo{
		ClusterID:   "c1",
		Status:      "running",
		AppEndpoint: "remote",
	}, nil)

	req := types.EndpointReq{
		ClusterID: "c1",
		Target:    "t1",
		Endpoint:  "http://127.0.0.1",
	}

	endpoint, host, err := ExtractDeployTargetAndHost(ctx, cc, req)
	require.Nil(t, err)
	require.Equal(t, "remote", endpoint)
	require.Equal(t, "127.0.0.1", host)
}

func TestClusterComponent_IndexPublic(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	// Mock cluster store to return clusters with different statuses
	cc.mocks.stores.ClusterInfoMock().EXPECT().List(ctx).Return([]database.ClusterInfo{
		{ClusterID: "c1", Status: "running", Enable: true, Region: "us-west-1"},
		{ClusterID: "c2", Status: "Unavailable", Enable: true, Region: "us-east-1"}, // Should be filtered out
		{ClusterID: "c3", Status: "running", Enable: false, Region: "eu-west-1"},    // Should be filtered out
		{ClusterID: "c4", Status: "running", Enable: true, Region: "ap-southeast-1"},
	}, nil)

	// Mock deployer for clusters that should be included
	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{
		ClusterID: "c1",
		Resources: []types.NodeResourceInfo{
			{
				NodeHardware: types.NodeHardware{
					GPUVendor: "NVIDIA",
					XPUModel:  "A100",
					XPUMem:    "40Gi"},
			},
			{
				NodeHardware: types.NodeHardware{
					GPUVendor: "AMD",
					XPUModel:  "MI100",
					XPUMem:    "32Gi"},
			},
		},
	}, nil)

	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c4").Return(&types.ClusterRes{
		ClusterID: "c4",
		Resources: []types.NodeResourceInfo{
			{NodeHardware: types.NodeHardware{
				GPUVendor: "NVIDIA",
				XPUModel:  "H100",
				XPUMem:    "80Gi"},
			},
		},
	}, nil)

	// Test the method
	result, err := cc.IndexPublic(ctx)
	require.Nil(t, err)

	// Verify results
	require.Len(t, result.Hardware, 3)
	require.Len(t, result.Regions, 2)
	require.Len(t, result.GPUVendors, 2)
	require.Contains(t, result.Regions, "us-west-1")
	require.Contains(t, result.Regions, "ap-southeast-1")
	require.Contains(t, result.GPUVendors, "NVIDIA")
	require.Contains(t, result.GPUVendors, "AMD")
}

func TestClusterComponent_IndexPublic_DeployerError(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	// Mock cluster store to return a cluster
	cc.mocks.stores.ClusterInfoMock().EXPECT().List(ctx).Return([]database.ClusterInfo{
		{ClusterID: "c1", Status: "running", Enable: true, Region: "us-west-1"},
	}, nil)

	// Mock deployer to return error
	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(nil, fmt.Errorf("deployer error"))

	// Test the method
	result, err := cc.IndexPublic(ctx)
	require.Nil(t, err)

	// Verify results - should include cluster but no hardware info
	require.Len(t, result.Hardware, 0)
	require.Len(t, result.Regions, 1)
	require.Len(t, result.GPUVendors, 0)
	require.Contains(t, result.Regions, "us-west-1")
}

func TestClusterComponent_IndexPublic_ClusterStoreError(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	// Mock cluster store to return error
	cc.mocks.stores.ClusterInfoMock().EXPECT().List(ctx).Return(nil, fmt.Errorf("store error"))

	// Test the method
	result, err := cc.IndexPublic(ctx)
	require.NotNil(t, err)
	require.Equal(t, types.PublicClusterRes{}, result)
}

func TestClusterComponent_IndexPublic_GPUMemParseError(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	// Mock cluster store to return a cluster
	cc.mocks.stores.ClusterInfoMock().EXPECT().List(ctx).Return([]database.ClusterInfo{
		{ClusterID: "c1", Status: "running", Enable: true, Region: "us-west-1"},
	}, nil)

	// Mock deployer with invalid XPUMem value
	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{
		ClusterID: "c1",
		Resources: []types.NodeResourceInfo{
			{NodeHardware: types.NodeHardware{
				GPUVendor: "NVIDIA",
				XPUModel:  "A100",
				XPUMem:    "invalid-mem"}, // Invalid memory format
			},
		},
	}, nil)

	// Test the method
	result, err := cc.IndexPublic(ctx)
	require.Nil(t, err)

	// Verify results - should include cluster with 0 memory
	require.Len(t, result.Hardware, 1)
	require.Len(t, result.Regions, 1)
	require.Len(t, result.GPUVendors, 1)
	require.Equal(t, int64(0), result.Hardware[0].XPUMem)
	require.Equal(t, "NVIDIA", result.Hardware[0].GPUVendor)
	require.Equal(t, "A100", result.Hardware[0].XPUModel)
	require.Equal(t, "us-west-1", result.Hardware[0].Region)
}
