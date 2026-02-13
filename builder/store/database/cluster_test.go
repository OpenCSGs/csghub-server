package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestClusterStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	_, err := store.Add(ctx, "foo", "bar", types.ConnectModeKubeConfig)
	require.Nil(t, err)

	cfg := &database.ClusterInfo{}
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", cfg.Region)

	// already exist, do nothing
	_, err = store.Add(ctx, "foo", "bar2", types.ConnectModeKubeConfig)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", cfg.Region)

	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = store.Update(ctx, database.ClusterInfo{
		ClusterID:     cfg.ClusterID,
		ClusterConfig: "foo",
		Region:        "bar3",
	})
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar3", cfg.Region)

	dbCluster := &database.ClusterInfo{}
	err = db.Core.NewSelect().Model(dbCluster).Limit(1).Scan(ctx)
	require.Nil(t, err)
	info, err := store.ByClusterID(ctx, dbCluster.ClusterID)
	require.Nil(t, err)
	require.Equal(t, "bar3", info.Region)

	infos, err := store.List(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(infos))
	require.Equal(t, "bar3", infos[0].Region)

}

func TestClusterStore_BatchUpdateStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	cls1, err := store.Add(ctx, "foo", "region1", types.ConnectModeKubeConfig)
	require.Nil(t, err)

	cls2, err := store.Add(ctx, "bar", "region2", types.ConnectModeKubeConfig)
	require.Nil(t, err)

	statusEvent := []*types.ClusterRes{
		{
			ClusterID: cls1.ClusterID,
			Status:    types.ClusterStatusRunning,
		},
		{
			ClusterID: cls2.ClusterID,
			Status:    types.ClusterStatusUnavailable,
		},
	}

	err = store.BatchUpdateStatus(ctx, statusEvent)
	require.Nil(t, err)

	c1, err := store.ByClusterID(ctx, cls1.ClusterID)
	require.Nil(t, err)
	require.Equal(t, types.ClusterStatusRunning, c1.Status)

	c2, err := store.ByClusterID(ctx, cls2.ClusterID)
	require.Nil(t, err)
	require.Equal(t, types.ClusterStatusRunning, c2.Status)
}

func TestClusterStore_ListAllNodes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	nodes, err := store.ListAllNodes(ctx)
	require.NoError(t, err)
	require.Empty(t, nodes)

	cluster1, err := store.Add(ctx, "config-1", "region-1", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	cluster2, err := store.Add(ctx, "config-2", "region-2", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	node1 := &database.ClusterNode{
		ClusterID: cluster1.ClusterID,
		Name:      "node-1",
		Status:    "Ready",
		Labels:    map[string]string{"zone": "a"},
		Hardware: types.NodeHardware{
			TotalXPU:  4,
			GPUVendor: "NVIDIA",
			XPUModel:  "A100",
			XPUMem:    "40GB",
		},
	}
	_, err = db.Core.NewInsert().Model(node1).Exec(ctx)
	require.NoError(t, err)

	node2 := &database.ClusterNode{
		ClusterID: cluster1.ClusterID,
		Name:      "node-2",
		Status:    "Ready",
		Labels:    map[string]string{"zone": "b"},
		Hardware: types.NodeHardware{
			TotalXPU:  2,
			GPUVendor: "NVIDIA",
			XPUModel:  "V100",
			XPUMem:    "32GB",
		},
	}
	_, err = db.Core.NewInsert().Model(node2).Exec(ctx)
	require.NoError(t, err)

	node3 := &database.ClusterNode{
		ClusterID: cluster2.ClusterID,
		Name:      "node-3",
		Status:    "NotReady",
		Labels:    map[string]string{"zone": "c"},
		Hardware: types.NodeHardware{
			TotalXPU: 0,
		},
	}
	_, err = db.Core.NewInsert().Model(node3).Exec(ctx)
	require.NoError(t, err)

	nodes, err = store.ListAllNodes(ctx)
	require.NoError(t, err)
	require.Len(t, nodes, 3)
}

func TestClusterStore_GetNodeByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	// Test getting non-existent node
	node, err := store.GetNodeByID(ctx, 99999)
	require.NoError(t, err)
	require.Nil(t, node)

	// Create a cluster
	cluster, err := store.Add(ctx, "test-config", "test-region", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	// Insert a node
	testNode := &database.ClusterNode{
		ClusterID:   cluster.ClusterID,
		Name:        "test-node",
		Status:      "Ready",
		Labels:      map[string]string{"zone": "a", "env": "test"},
		EnableVXPU:  true,
		ComputeCard: "2 x NVIDIA A100 40GB",
		Hardware: types.NodeHardware{
			TotalCPU:     32,
			AvailableCPU: 28,
			TotalMem:     256,
			AvailableMem: 200,
			XPUModel:     "A100",
			TotalXPU:     2,
			AvailableXPU: 1,
			GPUVendor:    "NVIDIA",
			XPUMem:       "40GB",
		},
	}
	_, err = db.Core.NewInsert().Model(testNode).Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, testNode.ID)

	// Get node by ID
	node, err = store.GetNodeByID(ctx, testNode.ID)
	require.NoError(t, err)
	require.NotNil(t, node)

	// Verify node data
	require.Equal(t, testNode.ID, node.ID)
	require.Equal(t, testNode.ClusterID, node.ClusterID)
	require.Equal(t, testNode.Name, node.Name)
	require.Equal(t, testNode.Status, node.Status)
	require.Equal(t, testNode.Labels, node.Labels)
	require.Equal(t, testNode.EnableVXPU, node.EnableVXPU)
	require.Equal(t, testNode.ComputeCard, node.ComputeCard)
	require.Equal(t, "test-region", node.ClusterRegion)

	// Verify hardware
	require.Equal(t, testNode.Hardware.TotalCPU, node.Hardware.TotalCPU)
	require.Equal(t, testNode.Hardware.AvailableCPU, node.Hardware.AvailableCPU)
	require.Equal(t, testNode.Hardware.TotalMem, node.Hardware.TotalMem)
	require.Equal(t, testNode.Hardware.AvailableMem, node.Hardware.AvailableMem)
	require.Equal(t, testNode.Hardware.XPUModel, node.Hardware.XPUModel)
	require.Equal(t, testNode.Hardware.TotalXPU, node.Hardware.TotalXPU)
	require.Equal(t, testNode.Hardware.AvailableXPU, node.Hardware.AvailableXPU)
	require.Equal(t, testNode.Hardware.GPUVendor, node.Hardware.GPUVendor)
	require.Equal(t, testNode.Hardware.XPUMem, node.Hardware.XPUMem)
}

func TestClusterStore_UpdateNode(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	node, err := store.UpdateNode(ctx, 99999, true)
	require.NoError(t, err)
	require.Nil(t, node)

	cluster, err := store.Add(ctx, "test-config", "test-region", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	testNode := &database.ClusterNode{
		ClusterID:  cluster.ClusterID,
		Name:       "test-node",
		Status:     "Ready",
		EnableVXPU: false,
		Hardware: types.NodeHardware{
			TotalCPU:     32,
			AvailableCPU: 28,
			TotalMem:     256,
			AvailableMem: 200,
			XPUModel:     "A100",
			TotalXPU:     2,
			AvailableXPU: 1,
			GPUVendor:    "NVIDIA",
			XPUMem:       "40GB",
		},
	}
	_, err = db.Core.NewInsert().Model(testNode).Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, testNode.ID)

	updatedNode, err := store.UpdateNode(ctx, testNode.ID, true)
	require.NoError(t, err)
	require.NotNil(t, updatedNode)
	require.Equal(t, testNode.ID, updatedNode.ID)
	require.True(t, updatedNode.EnableVXPU)
	require.Equal(t, "test-node", updatedNode.Name)
}
