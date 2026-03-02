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

func TestArgoWorkflowStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewArgoWorkFlowStoreWithDB(db)

	dt := time.Date(2022, 1, 1, 1, 1, 0, 0, time.UTC)
	_, err := store.CreateWorkFlow(ctx, database.ArgoWorkflow{
		Username:   "user",
		Namespace:  "ns",
		TaskName:   "task",
		TaskId:     "tid",
		TaskType:   types.TaskTypeEvaluation,
		SubmitTime: dt,
	})
	require.Nil(t, err)

	dbflow := &database.ArgoWorkflow{}
	err = db.Core.NewSelect().Model(dbflow).Where("task_name=?", "task").Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, "ns", dbflow.Namespace)
	require.Equal(t, "task", dbflow.TaskName)

	dbflow.TaskName = "task-new"
	_, err = store.UpdateWorkFlow(ctx, *dbflow)
	require.Nil(t, err)

	dbflow = &database.ArgoWorkflow{}
	err = db.Core.NewSelect().Model(dbflow).Where("task_id=?", "tid").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "task-new", dbflow.TaskName)

	flowfind, err := store.FindByID(ctx, dbflow.ID)
	require.Nil(t, err)
	require.Equal(t, "task-new", flowfind.TaskName)

	flowfind, err = store.FindByTaskID(ctx, "tid")
	require.Nil(t, err)
	require.Equal(t, "task-new", flowfind.TaskName)

	_, err = store.CreateWorkFlow(ctx, database.ArgoWorkflow{
		Username:   "user",
		Namespace:  "ns",
		TaskName:   "task2",
		TaskId:     "tid2",
		TaskType:   types.TaskTypeEvaluation,
		SubmitTime: dt.Add(-5 * time.Hour),
	})
	require.Nil(t, err)
	flows, total, err := store.FindByUsername(ctx, "user", types.TaskTypeEvaluation, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	names := []string{}
	for _, f := range flows {
		names = append(names, f.TaskName)
	}
	require.Equal(t, []string{"task-new", "task2"}, names)

	err = store.DeleteWorkFlow(ctx, dbflow.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, dbflow.ID)
	require.NotNil(t, err)

}

func TestArgoWorkflowStore_GetClusterWorkflows(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewArgoWorkFlowStoreWithDB(db)
	dt := time.Date(2022, 1, 1, 1, 1, 0, 0, time.UTC)

	// Create test workflows with different cluster IDs, nodes, statuses, and resource names
	workflows := []database.ArgoWorkflow{
		{
			Username:     "user1",
			UserUUID:     "uuid1",
			Namespace:    "ns1",
			TaskName:     "task-alpha",
			TaskId:       "tid-001",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "cluster-1",
			ClusterNode:  "node-1,node-2",
			Status:       "Running",
			ResourceName: "gpu-small",
			Image:        "image1",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			SubmitTime:   dt,
		},
		{
			Username:     "user2",
			UserUUID:     "uuid2",
			Namespace:    "ns2",
			TaskName:     "task-beta",
			TaskId:       "tid-002",
			TaskType:     types.TaskTypeTraining,
			ClusterID:    "cluster-1",
			ClusterNode:  "node-2,node-3",
			Status:       "Succeeded",
			ResourceName: "gpu-large",
			Image:        "image2",
			RepoIds:      []string{"repo2"},
			RepoType:     "dataset",
			SubmitTime:   dt.Add(time.Hour),
		},
		{
			Username:     "test-user",
			UserUUID:     "uuid3",
			Namespace:    "ns3",
			TaskName:     "task-gamma",
			TaskId:       "tid-003",
			TaskType:     types.TaskTypeFinetune,
			ClusterID:    "cluster-2",
			ClusterNode:  "node-1",
			Status:       "Running",
			ResourceName: "gpu-small",
			Image:        "image3",
			RepoIds:      []string{"repo3"},
			RepoType:     "model",
			SubmitTime:   dt.Add(2 * time.Hour),
		},
		{
			Username:     "user1",
			UserUUID:     "uuid4",
			Namespace:    "ns4",
			TaskName:     "search-task",
			TaskId:       "tid-004",
			TaskType:     types.TaskTypeComparison,
			ClusterID:    "cluster-1",
			ClusterNode:  "",
			Status:       "Failed",
			ResourceName: "cpu-small",
			Image:        "image4",
			RepoIds:      []string{"repo4"},
			RepoType:     "space",
			SubmitTime:   dt.Add(3 * time.Hour),
		},
	}

	for _, wf := range workflows {
		_, err := store.CreateWorkFlow(ctx, wf)
		require.Nil(t, err)
	}

	// Test 1: Get all workflows with pagination
	req := types.ClusterWFReq{
		Per:  10,
		Page: 1,
	}
	result, total, err := store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total)
	require.Equal(t, 4, len(result))

	// Test 2: Filter by ClusterID
	req = types.ClusterWFReq{
		ClusterID: "cluster-1",
		Per:       10,
		Page:      1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, 3, len(result))

	// Test 3: Filter by ClusterNode
	req = types.ClusterWFReq{
		ClusterNode: "node-1",
		Per:         10,
		Page:        1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(result))

	// Test 4: Filter by Status
	req = types.ClusterWFReq{
		Status: "Running",
		Per:    10,
		Page:   1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(result))
	for _, wf := range result {
		require.Equal(t, "Running", string(wf.Status))
	}

	// Test 5: Filter by ResourceName
	req = types.ClusterWFReq{
		ResourceName: "gpu-small",
		Per:          10,
		Page:         1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(result))

	// Test 6: Search by task name
	req = types.ClusterWFReq{
		Search: "alpha",
		Per:    10,
		Page:   1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(result))
	require.Equal(t, "task-alpha", result[0].TaskName)

	// Test 7: Search by username
	req = types.ClusterWFReq{
		Search: "user1",
		Per:    10,
		Page:   1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(result))

	// Test 8: Combined filters (ClusterID + Status)
	req = types.ClusterWFReq{
		ClusterID: "cluster-1",
		Status:    "Running",
		Per:       10,
		Page:      1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(result))
	require.Equal(t, "cluster-1", result[0].ClusterID)
	require.Equal(t, "Running", string(result[0].Status))

	// Test 9: Pagination - Page 1 with Per=2
	req = types.ClusterWFReq{
		Per:  2,
		Page: 1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total)
	require.Equal(t, 2, len(result))

	// Test 10: Pagination - Page 2 with Per=2
	req = types.ClusterWFReq{
		Per:  2,
		Page: 2,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total)
	require.Equal(t, 2, len(result))

	// Test 11: No results for non-existent cluster
	req = types.ClusterWFReq{
		ClusterID: "non-existent-cluster",
		Per:       10,
		Page:      1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, 0, len(result))

	// Test 12: Order by ID DESC (default)
	req = types.ClusterWFReq{
		Per:  10,
		Page: 1,
	}
	result, total, err = store.GetClusterWorkflows(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total)
	// Results should be in descending order by ID
	require.True(t, result[0].ID > result[1].ID)
	require.True(t, result[1].ID > result[2].ID)
	require.True(t, result[2].ID > result[3].ID)
}
