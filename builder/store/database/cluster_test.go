package database_test

import (
	"context"
	"testing"
	"time"

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

	deployStore := database.NewDeployTaskStoreWithDB(db)

	err = deployStore.CreateDeploy(ctx, &database.Deploy{
		ClusterID:  cls1.ClusterID,
		DeployName: "deploy-1",
		SvcName:    "svc-1",
	})
	require.Nil(t, err)

	err = deployStore.CreateDeploy(ctx, &database.Deploy{
		ClusterID:  cls2.ClusterID,
		DeployName: "deploy-2",
		SvcName:    "svc-2",
	})
	require.Nil(t, err)

	workflowStore := database.NewArgoWorkFlowStoreWithDB(db)
	_, err = workflowStore.CreateWorkFlow(ctx, database.ArgoWorkflow{
		ClusterID: cls2.ClusterID,
		TaskId:    "workflow-1",
	})
	require.Nil(t, err)

	statusEvent := []*types.ClusterRes{
		{
			ClusterID: cls1.ClusterID,
			Status:    types.ClusterStatusRunning,
			Resources: []types.NodeResourceInfo{
				{
					Processes: []types.ProcessInfo{
						{
							DeployID:     "deploy-1",
							SvcName:      "svc-1",
							WorkflowName: "",
							ClusterNode:  "node-1",
						},
						{
							DeployID:     "deploy-2",
							SvcName:      "svc-2",
							WorkflowName: "",
							ClusterNode:  "node-1",
						},
					},
				},
				{
					Processes: []types.ProcessInfo{
						{
							DeployID:     "deploy-2",
							SvcName:      "svc-2",
							WorkflowName: "",
							ClusterNode:  "node-2",
						},
					},
				},
			},
		},
		{
			ClusterID: cls2.ClusterID,
			Status:    types.ClusterStatusUnavailable,
			Resources: []types.NodeResourceInfo{
				{
					Processes: []types.ProcessInfo{
						{
							DeployID:     "workflow-1",
							SvcName:      "",
							WorkflowName: "workflow-1",
							ClusterNode:  "node-3",
						},
					},
				},
			},
		},
	}

	err = store.BatchUpdateStatus(ctx, statusEvent, time.Now())
	require.Nil(t, err)

	c1, err := store.ByClusterID(ctx, cls1.ClusterID)
	require.Nil(t, err)
	require.Equal(t, types.ClusterStatusRunning, c1.Status)

	c2, err := store.ByClusterID(ctx, cls2.ClusterID)
	require.Nil(t, err)
	require.Equal(t, types.ClusterStatusRunning, c2.Status)

	deploy1, err := deployStore.GetDeployBySvcName(ctx, "svc-1")
	require.Nil(t, err)
	require.Equal(t, "node-1", deploy1.ClusterNode)

	deploy2, err := deployStore.GetDeployBySvcName(ctx, "svc-2")
	require.Nil(t, err)
	require.Equal(t, "node-1,node-2", deploy2.ClusterNode)

	workflow1, err := workflowStore.FindByTaskID(ctx, "workflow-1")
	require.Nil(t, err)
	require.Equal(t, "node-3", workflow1.ClusterNode)
}

func TestClusterStore_UpdateByClusterID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	// Insert new cluster
	clusterInfo1 := types.ClusterEvent{
		ClusterID: "cluster-1",
		Region:    "region-a",
		Zone:      "zone-a",
	}
	err := store.UpdateByClusterID(ctx, clusterInfo1)
	require.NoError(t, err)

	// Verify insertion
	persistedCluster1, err := store.ByClusterID(ctx, "cluster-1")
	require.NoError(t, err)
	require.Equal(t, "region-a", persistedCluster1.Region)
	require.Equal(t, "zone-a", persistedCluster1.Zone)
	require.True(t, persistedCluster1.Enable)

	// Update existing cluster
	clusterInfo2 := types.ClusterEvent{
		ClusterID: "cluster-1",
		Region:    "region-b",
		Zone:      "zone-b",
	}
	err = store.UpdateByClusterID(ctx, clusterInfo2)
	require.NoError(t, err)

	// Verify update
	persistedCluster2, err := store.ByClusterID(ctx, "cluster-1")
	require.NoError(t, err)
	require.Equal(t, "region-b", persistedCluster2.Region)
	require.Equal(t, "zone-b", persistedCluster2.Zone)
	require.True(t, persistedCluster2.Enable)
}

func TestClusterStore_BatchUpdateStatus_OfflineTimeout(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	// Create a cluster
	cluster, err := store.Add(ctx, "test-config", "test-region", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	// Create a node with "Ready" status
	node := &database.ClusterNode{
		ClusterID: cluster.ClusterID,
		Name:      "test-node",
		Status:    "Ready",
	}
	_, err = db.Core.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, node.ID)

	// Manually set the node's updated_at to more than 240 seconds ago
	oldTime := time.Now().Add(-250 * time.Second)
	_, err = db.Core.NewUpdate().Model(&database.ClusterNode{}).
		Set("updated_at = ?", oldTime).
		Where("id = ?", node.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Call BatchUpdateStatus (which should trigger the offline timeout logic)
	// Even with empty status event, the timeout logic should run
	statusEvent := []*types.ClusterRes{
		{
			ClusterID: cluster.ClusterID,
			Status:    types.ClusterStatusRunning,
			Resources: []types.NodeResourceInfo{},
		},
	}
	err = store.BatchUpdateStatus(ctx, statusEvent, time.Now())
	require.NoError(t, err)

	// Verify the node status is updated to "Offline"
	updatedNode, err := store.GetClusterNodeByID(ctx, node.ID)
	require.NoError(t, err)
	require.Equal(t, string(types.NodeStatusOffline), updatedNode.Status)
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

	_, err = db.Core.NewUpdate().Model(&database.ClusterInfo{}).Set("status = ?", types.ClusterStatusRunning).Where("cluster_id = ?", cluster1.ClusterID).Exec(ctx)
	require.NoError(t, err)

	_, err = db.Core.NewUpdate().Model(&database.ClusterInfo{}).Set("status = ?", types.ClusterStatusUnavailable).Where("cluster_id = ?", cluster2.ClusterID).Exec(ctx)
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

	nodeRegions := make(map[string]string)
	for _, node := range nodes {
		nodeRegions[node.ClusterID] = node.ClusterRegion
	}
	require.Equal(t, "region-1", nodeRegions[cluster1.ClusterID])
	require.Equal(t, "region-2", nodeRegions[cluster2.ClusterID])
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

func TestClusterStore_ClusterNodeOperations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	// 1. Create a cluster
	cluster, err := store.Add(ctx, "config-1", "region-1", types.ConnectModeKubeConfig)
	require.NoError(t, err)

	// 2. Create a cluster node directly in DB
	node := &database.ClusterNode{
		ClusterID: cluster.ClusterID,
		Name:      "node-1",
		Status:    "Ready",
		Exclusive: false,
	}
	_, err = db.Core.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, node.ID)

	// 3. Test GetClusterNodeByID
	fetchedNode, err := store.GetClusterNodeByID(ctx, node.ID)
	require.NoError(t, err)
	require.Equal(t, node.Name, fetchedNode.Name)
	require.Equal(t, node.ClusterID, fetchedNode.ClusterID)
	require.False(t, fetchedNode.Exclusive)

	// 4. Test UpdateClusterNode
	fetchedNode.Exclusive = true
	err = store.UpdateClusterNodeByNode(ctx, *fetchedNode)
	require.NoError(t, err)

	updatedNode, err := store.GetClusterNodeByID(ctx, node.ID)
	require.NoError(t, err)
	require.True(t, updatedNode.Exclusive)

	// 5. Test AddNodeOwnership
	ownership := database.ClusterNodeOwnership{
		ClusterNodeID: node.ID,
		ClusterID:     cluster.ClusterID,
		Namespace:     "ns-1",
	}
	err = store.AddNodeOwnership(ctx, ownership)
	require.NoError(t, err)

	// 6. Test GetNodeOwnership
	fetchedOwnership, err := store.GetNodeOwnership(ctx, node.ID)
	require.NoError(t, err)
	require.NotNil(t, fetchedOwnership)
	require.Equal(t, ownership.Namespace, fetchedOwnership.Namespace)
	require.Equal(t, ownership.ClusterNodeID, fetchedOwnership.ClusterNodeID)

	// 7. Test DeleteNodeOwnership
	err = store.DeleteNodeOwnership(ctx, node.ID)
	require.NoError(t, err)

	deletedOwnership, err := store.GetNodeOwnership(ctx, node.ID)
	require.NoError(t, err)
	require.Nil(t, deletedOwnership)
}
