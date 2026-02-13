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

	err = store.BatchUpdateStatus(ctx, statusEvent)
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
