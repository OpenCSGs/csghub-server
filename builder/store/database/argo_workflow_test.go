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

func TestArgoWorkflowStore_ListWorkflowsByTimeRange(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewArgoWorkFlowStoreWithDB(db)

	// Create workflows with different submit times
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	workflows := []database.ArgoWorkflow{
		{
			Username:   "user1",
			Namespace:  "ns1",
			TaskName:   "task-1",
			TaskId:     "tid-time-001",
			TaskType:   types.TaskTypeEvaluation,
			ClusterID:  "cluster-1",
			SubmitTime: baseTime.Add(-48 * time.Hour), // 2024-01-13 10:00:00
		},
		{
			Username:   "user2",
			Namespace:  "ns2",
			TaskName:   "task-2",
			TaskId:     "tid-time-002",
			TaskType:   types.TaskTypeTraining,
			ClusterID:  "cluster-1",
			SubmitTime: baseTime.Add(-24 * time.Hour), // 2024-01-14 10:00:00
		},
		{
			Username:   "user1",
			Namespace:  "ns3",
			TaskName:   "task-3",
			TaskId:     "tid-time-003",
			TaskType:   types.TaskTypeEvaluation,
			ClusterID:  "cluster-2",
			SubmitTime: baseTime, // 2024-01-15 10:00:00
		},
		{
			Username:   "user3",
			Namespace:  "ns4",
			TaskName:   "task-4",
			TaskId:     "tid-time-004",
			TaskType:   types.TaskTypeFinetune,
			ClusterID:  "cluster-1",
			SubmitTime: baseTime.Add(24 * time.Hour), // 2024-01-16 10:00:00
		},
		{
			Username:   "user2",
			Namespace:  "ns5",
			TaskName:   "task-5",
			TaskId:     "tid-time-005",
			TaskType:   types.TaskTypeComparison,
			ClusterID:  "cluster-2",
			SubmitTime: baseTime.Add(48 * time.Hour), // 2024-01-17 10:00:00
		},
	}

	for _, wf := range workflows {
		_, err := store.CreateWorkFlow(ctx, wf)
		require.Nil(t, err)
	}

	// Test 1: Get all workflows with no time filter (should return all)
	req := types.WorkflowTimeRangeReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err := store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Equal(t, 5, len(result))

	// Test 2: Filter by start time only
	startTime := baseTime.Add(-12 * time.Hour) // 2024-01-15 22:00:00
	req = types.WorkflowTimeRangeReq{
		StartTime: &startTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 3, total) // task-3, task-4, task-5
	require.Equal(t, 3, len(result))

	// Test 3: Filter by end time only
	endTime := baseTime.Add(12 * time.Hour) // 2024-01-15 22:00:00
	req = types.WorkflowTimeRangeReq{
		EndTime: &endTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 3, total) // task-1, task-2, task-3
	require.Equal(t, 3, len(result))

	// Test 4: Filter by both start and end time (middle range)
	startTime = baseTime.Add(-12 * time.Hour)
	endTime = baseTime.Add(36 * time.Hour)
	req = types.WorkflowTimeRangeReq{
		StartTime: &startTime,
		EndTime:   &endTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 2, total) // task-3, task-4
	require.Equal(t, 2, len(result))

	// Test 5: No results - outside time range
	startTime = baseTime.Add(100 * time.Hour)
	endTime = baseTime.Add(200 * time.Hour)
	req = types.WorkflowTimeRangeReq{
		StartTime: &startTime,
		EndTime:   &endTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, 0, len(result))

	// Test 6: Pagination - Page 1 with PageSize=2
	req = types.WorkflowTimeRangeReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 2,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Equal(t, 2, len(result))

	// Test 7: Pagination - Page 2 with PageSize=2
	req = types.WorkflowTimeRangeReq{
		PageOpts: types.PageOpts{
			Page:     2,
			PageSize: 2,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Equal(t, 2, len(result))

	// Test 8: Pagination - Page 3 with PageSize=2 (last page)
	req = types.WorkflowTimeRangeReq{
		PageOpts: types.PageOpts{
			Page:     3,
			PageSize: 2,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Equal(t, 1, len(result))

	// Test 9: Order by submit_time DESC (default)
	req = types.WorkflowTimeRangeReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	result, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	// Results should be in descending order by submit_time
	require.True(t, result[0].SubmitTime.After(result[1].SubmitTime) || result[0].SubmitTime.Equal(result[1].SubmitTime))
	require.True(t, result[1].SubmitTime.After(result[2].SubmitTime) || result[1].SubmitTime.Equal(result[2].SubmitTime))
	require.True(t, result[2].SubmitTime.After(result[3].SubmitTime) || result[2].SubmitTime.Equal(result[3].SubmitTime))
	require.True(t, result[3].SubmitTime.After(result[4].SubmitTime) || result[3].SubmitTime.Equal(result[4].SubmitTime))

	// Test 10: Exact time boundary - start time equals workflow submit time
	startTime = baseTime.Add(-24 * time.Hour) // Exactly when task-2 was submitted
	req = types.WorkflowTimeRangeReq{
		StartTime: &startTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	_, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total) // task-2, task-3, task-4, task-5 (not task-1)

	// Test 11: Exact time boundary - end time equals workflow submit time
	endTime = baseTime.Add(24 * time.Hour) // Exactly when task-4 was submitted
	req = types.WorkflowTimeRangeReq{
		EndTime: &endTime,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	_, total, err = store.ListWorkflowsByTimeRange(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 4, total) // task-1, task-2, task-3, task-4 (not task-5)
}

func TestArgoWorkflowStore_ListRunningWorkflowsByUserUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewArgoWorkFlowStoreWithDB(db)
	dt := time.Date(2022, 1, 1, 1, 1, 0, 0, time.UTC)

	workflows := []database.ArgoWorkflow{
		{
			Username:   "user1",
			UserUUID:   "uuid-user-1",
			Namespace:  "ns1",
			TaskName:   "task-eval-1",
			TaskId:     "tid-run-001",
			TaskType:   types.TaskTypeEvaluation,
			ClusterID:  "cluster-1",
			Status:     "Running",
			Image:      "image1",
			RepoIds:    []string{"repo1"},
			RepoType:   "model",
			SubmitTime: dt,
		},
		{
			Username:   "user1",
			UserUUID:   "uuid-user-1",
			Namespace:  "ns2",
			TaskName:   "task-train-1",
			TaskId:     "tid-run-002",
			TaskType:   types.TaskTypeTraining,
			ClusterID:  "cluster-1",
			Status:     "Running",
			Image:      "image2",
			RepoIds:    []string{"repo2"},
			RepoType:   "dataset",
			SubmitTime: dt.Add(time.Hour),
		},
		{
			Username:   "user1",
			UserUUID:   "uuid-user-1",
			Namespace:  "ns3",
			TaskName:   "task-eval-stopped",
			TaskId:     "tid-stop-001",
			TaskType:   types.TaskTypeEvaluation,
			ClusterID:  "cluster-2",
			Status:     "Succeeded",
			Image:      "image3",
			RepoIds:    []string{"repo3"},
			RepoType:   "model",
			SubmitTime: dt.Add(2 * time.Hour),
		},
		{
			Username:   "user2",
			UserUUID:   "uuid-user-2",
			Namespace:  "ns4",
			TaskName:   "task-finetune-1",
			TaskId:     "tid-run-003",
			TaskType:   types.TaskTypeFinetune,
			ClusterID:  "cluster-1",
			Status:     "Running",
			Image:      "image4",
			RepoIds:    []string{"repo4"},
			RepoType:   "model",
			SubmitTime: dt.Add(3 * time.Hour),
		},
		{
			Username:   "user2",
			UserUUID:   "uuid-user-2",
			Namespace:  "ns5",
			TaskName:   "task-eval-2",
			TaskId:     "tid-fail-001",
			TaskType:   types.TaskTypeEvaluation,
			ClusterID:  "cluster-2",
			Status:     "Failed",
			Image:      "image5",
			RepoIds:    []string{"repo5"},
			RepoType:   "space",
			SubmitTime: dt.Add(4 * time.Hour),
		},
	}

	for _, wf := range workflows {
		_, err := store.CreateWorkFlow(ctx, wf)
		require.Nil(t, err)
	}

	result, err := store.ListRunningWorkflowsByUserUUID(ctx, "uuid-user-1")
	require.Nil(t, err)
	require.Equal(t, 2, len(result))

	result, err = store.ListRunningWorkflowsByUserUUID(ctx, "uuid-user-2")
	require.Nil(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, "task-finetune-1", result[0].TaskName)

	result, err = store.ListRunningWorkflowsByUserUUID(ctx, "non-existent-user")
	require.Nil(t, err)
	require.Equal(t, 0, len(result))

	result, err = store.ListRunningWorkflowsByUserUUID(ctx, "uuid-user-1")
	require.Nil(t, err)
	for _, wf := range result {
		require.Equal(t, "Running", string(wf.Status))
	}
}
