package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mockDeploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
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

	mockDeployer := mockDeploy.NewMockDeployer(t)

	// Create cluster component with mock
	c := &clusterComponentImpl{deployer: mockDeployer}

	// Test the method
	mockDeployer.EXPECT().ListCluster(context.Background()).Return([]types.ClusterRes{
		{ClusterID: "c1"},
		{ClusterID: "c2"},
	}, nil)
	mockDeployer.EXPECT().GetClusterUsageById(context.Background(), "c1").Return(&types.ClusterRes{
		ClusterID: "c1",
		Status:    types.ClusterStatusRunning,
	}, nil)
	mockDeployer.EXPECT().GetClusterUsageById(context.Background(), "c2").Return(&types.ClusterRes{
		ClusterID: "c2",
		Status:    types.ClusterStatusRunning,
	}, nil)
	res, err := c.GetClusterUsages(context.Background())

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
