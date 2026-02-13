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
		UserUUID:      "user-1",
		OrgUUID:       "org-1",
	}
	err = store.AddNodeOwnership(ctx, ownership)
	require.NoError(t, err)

	// 6. Test GetNodeOwnership
	fetchedOwnership, err := store.GetNodeOwnership(ctx, node.ID)
	require.NoError(t, err)
	require.NotNil(t, fetchedOwnership)
	require.Equal(t, ownership.UserUUID, fetchedOwnership.UserUUID)
	require.Equal(t, ownership.OrgUUID, fetchedOwnership.OrgUUID)
	require.Equal(t, ownership.ClusterNodeID, fetchedOwnership.ClusterNodeID)

	// 7. Test DeleteNodeOwnership
	err = store.DeleteNodeOwnership(ctx, node.ID)
	require.NoError(t, err)

	deletedOwnership, err := store.GetNodeOwnership(ctx, node.ID)
	require.NoError(t, err)
	require.Nil(t, deletedOwnership)
}
