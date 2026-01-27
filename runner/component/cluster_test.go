package component

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
	mockCluster "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/cluster"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestClusterComponent_ByClusterID_Success(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	expectedClusterInfo := database.ClusterInfo{
		ClusterID:     "test-cluster-id",
		ClusterConfig: "config",
		Region:        "us-west-1",
		Zone:          "us-west-1a",
		Provider:      "aws",
		Enable:        true,
		StorageClass:  "gp3",
	}

	clusterStore.EXPECT().ByClusterID(mock.Anything, "test-cluster-id").Return(expectedClusterInfo, nil)

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result, err := clusterComponent.ByClusterID(ctx, "test-cluster-id")

	require.Nil(t, err)
	require.Equal(t, expectedClusterInfo.ClusterID, result.ClusterID)
}

func TestClusterComponent_ByClusterID_Error(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterStore.EXPECT().ByClusterID(mock.Anything, "test-cluster-id").Return(database.ClusterInfo{}, fmt.Errorf("cluster not found"))

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result, err := clusterComponent.ByClusterID(ctx, "test-cluster-id")

	require.Error(t, err)
	require.Equal(t, database.ClusterInfo{}, result)
}

func TestClusterComponent_GetResourceByID_Success(t *testing.T) {
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)

	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()

	testCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test-cluster-id",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-west-1",
		StorageClass:  "gp3",
	}

	pool.EXPECT().GetClusterByID(mock.Anything, "test-cluster-id").Return(testCluster, nil)

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: mockdb.NewMockClusterInfoStore(t),
		clusterPool:  pool,
	}

	resourceStatus, nodeResources, err := clusterComponent.GetResourceByID(ctx, "test-cluster-id")

	require.Nil(t, err)
	require.NotEmpty(t, resourceStatus)
	require.NotNil(t, nodeResources)
}

func TestClusterComponent_GetResourceByID_NotFound(t *testing.T) {
	ctx := context.TODO()
	pool := mockCluster.NewMockPool(t)

	pool.EXPECT().GetClusterByID(mock.Anything, "non-existent-id").Return(nil, fmt.Errorf("cluster not found"))

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: mockdb.NewMockClusterInfoStore(t),
		clusterPool:  pool,
	}

	resourceStatus, nodeResources, err := clusterComponent.GetResourceByID(ctx, "non-existent-id")

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to find cluster")
	require.Empty(t, resourceStatus)
	require.Nil(t, nodeResources)
}

func TestClusterComponent_collectResourceByID_Success(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterInfo := database.ClusterInfo{
		ClusterID:     "test-cluster-id",
		ClusterConfig: "config",
		Region:        "us-west-1",
		Zone:          "us-west-1a",
		Provider:      "aws",
		Enable:        true,
		StorageClass:  "gp3",
	}

	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()

	testCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test-cluster-id",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-west-1",
		StorageClass:  "gp3",
	}

	clusterStore.EXPECT().ByClusterID(mock.Anything, "test-cluster-id").Return(clusterInfo, nil)
	pool.EXPECT().GetClusterByID(mock.Anything, "test-cluster-id").Return(testCluster, nil)

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result, err := clusterComponent.collectResourceByID(ctx, "test-cluster-id")

	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, "test-cluster-id", result.ClusterID)
	require.Equal(t, "us-west-1", result.Region)
	require.Equal(t, "us-west-1a", result.Zone)
	require.Equal(t, "aws", result.Provider)
	require.Equal(t, "gp3", result.StorageClass)
	require.True(t, result.Enable)
}

func TestClusterComponent_collectResourceByID_GetClusterInfoError(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterStore.EXPECT().ByClusterID(mock.Anything, "test-cluster-id").Return(database.ClusterInfo{}, fmt.Errorf("database error"))

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result, err := clusterComponent.collectResourceByID(ctx, "test-cluster-id")

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to get cluster by clusterId")
}

func TestClusterComponent_collectResourceByID_GetResourceError(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterInfo := database.ClusterInfo{
		ClusterID:     "test-cluster-id",
		ClusterConfig: "config",
		Region:        "us-west-1",
		Zone:          "us-west-1a",
		Provider:      "aws",
		Enable:        true,
		StorageClass:  "gp3",
	}

	clusterStore.EXPECT().ByClusterID(mock.Anything, "test-cluster-id").Return(clusterInfo, nil)
	pool.EXPECT().GetClusterByID(mock.Anything, "test-cluster-id").Return(nil, fmt.Errorf("cluster not found"))

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result, err := clusterComponent.collectResourceByID(ctx, "test-cluster-id")

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to collect cluster physical resource")
}

func TestClusterComponent_collectAllClusters_Success(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterInfo1 := database.ClusterInfo{
		ClusterID:     "cluster-1",
		ClusterConfig: "config1",
		Region:        "us-west-1",
		Zone:          "us-west-1a",
		Provider:      "aws",
		Enable:        true,
		StorageClass:  "gp3",
	}

	clusterInfo2 := database.ClusterInfo{
		ClusterID:     "cluster-2",
		ClusterConfig: "config2",
		Region:        "us-east-1",
		Zone:          "us-east-1a",
		Provider:      "gcp",
		Enable:        true,
		StorageClass:  "standard",
	}

	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()

	testCluster1 := &cluster.Cluster{
		CID:           "config1",
		ID:            "cluster-1",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-west-1",
		StorageClass:  "gp3",
	}

	testCluster2 := &cluster.Cluster{
		CID:           "config2",
		ID:            "cluster-2",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-east-1",
		StorageClass:  "standard",
	}

	pool.EXPECT().GetAllCluster().Return([]*cluster.Cluster{testCluster1, testCluster2})
	clusterStore.EXPECT().ByClusterID(mock.Anything, "cluster-1").Return(clusterInfo1, nil)
	clusterStore.EXPECT().ByClusterID(mock.Anything, "cluster-2").Return(clusterInfo2, nil)
	pool.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(testCluster1, nil)
	pool.EXPECT().GetClusterByID(mock.Anything, "cluster-2").Return(testCluster2, nil)

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result := clusterComponent.collectAllClusters(ctx)

	require.NotNil(t, result)
	require.Len(t, result, 2)
	require.Equal(t, "cluster-1", result[0].ClusterID)
	require.Equal(t, "cluster-2", result[1].ClusterID)
}

func TestClusterComponent_collectAllClusters_ErrorContinue(t *testing.T) {
	ctx := context.TODO()
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	pool := mockCluster.NewMockPool(t)

	clusterInfo1 := database.ClusterInfo{
		ClusterID:     "cluster-1",
		ClusterConfig: "config1",
		Region:        "us-west-1",
		Zone:          "us-west-1a",
		Provider:      "aws",
		Enable:        true,
		StorageClass:  "gp3",
	}

	kubeClient := fake.NewSimpleClientset()
	knativeClient := knativefake.NewSimpleClientset()

	testCluster1 := &cluster.Cluster{
		CID:           "config1",
		ID:            "cluster-1",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-west-1",
		StorageClass:  "gp3",
	}

	testCluster2 := &cluster.Cluster{
		CID:           "config2",
		ID:            "cluster-2",
		Client:        kubeClient,
		KnativeClient: knativeClient,
		ConnectMode:   types.ConnectModeInCluster,
		Region:        "us-east-1",
		StorageClass:  "standard",
	}

	pool.EXPECT().GetAllCluster().Return([]*cluster.Cluster{testCluster1, testCluster2})
	clusterStore.EXPECT().ByClusterID(mock.Anything, "cluster-1").Return(clusterInfo1, nil)
	clusterStore.EXPECT().ByClusterID(mock.Anything, "cluster-2").Return(database.ClusterInfo{}, fmt.Errorf("not found"))
	pool.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(testCluster1, nil)
	// pool.EXPECT().GetClusterByID(mock.Anything, "cluster-2").Return(nil, fmt.Errorf("not found"))

	clusterComponent := &clusterComponentImpl{
		env:          &config.Config{},
		clusterStore: clusterStore,
		clusterPool:  pool,
	}

	result := clusterComponent.collectAllClusters(ctx)

	require.NotNil(t, result)
	require.Len(t, result, 1)
	require.Equal(t, "cluster-1", result[0].ClusterID)
}
