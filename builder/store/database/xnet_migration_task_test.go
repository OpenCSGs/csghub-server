package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestXnetMigrationTaskStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewXnetMigrationTaskStoreWithDB(db)

	// Test CreateXnetMigrationTask
	err := store.CreateXnetMigrationTask(ctx, 123, "Initial task")
	require.Nil(t, err)

	// Test ListXnetMigrationTasksByRepoID
	tasks, err := store.ListXnetMigrationTasksByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, int64(123), tasks[0].RepositoryID)
	require.Equal(t, "Initial task", tasks[0].LastMessage)
	require.Equal(t, types.XnetMigrationTaskStatusPending, tasks[0].Status)

	taskID := tasks[0].ID

	// Test GetXnetMigrationTaskByID
	task, err := store.GetXnetMigrationTaskByID(ctx, taskID)
	require.Nil(t, err)
	require.Equal(t, taskID, task.ID)
	require.Equal(t, int64(123), task.RepositoryID)

	// Test UpdateXnetMigrationTask
	err = store.UpdateXnetMigrationTask(ctx, taskID, "Updated message", types.XnetMigrationTaskStatusRunning)
	require.Nil(t, err)

	// Verify update
	task, err = store.GetXnetMigrationTaskByID(ctx, taskID)
	require.Nil(t, err)
	require.Equal(t, "Updated message", task.LastMessage)
	require.Equal(t, types.XnetMigrationTaskStatusRunning, task.Status)

	// Create more tasks with different statuses
	err = store.CreateXnetMigrationTask(ctx, 456, "Task 2")
	require.Nil(t, err)
	tasks, err = store.ListXnetMigrationTasksByRepoID(ctx, 456)
	require.Nil(t, err)
	require.Len(t, tasks, 1)
	err = store.UpdateXnetMigrationTask(ctx, tasks[0].ID, "Task 2 completed", types.XnetMigrationTaskStatusCompleted)
	require.Nil(t, err)

	// Test ListXnetMigrationTasksByStatus
	runningTasks, err := store.ListXnetMigrationTasksByStatus(ctx, types.XnetMigrationTaskStatusRunning)
	require.Nil(t, err)
	require.Len(t, runningTasks, 1)
	require.Equal(t, "Updated message", runningTasks[0].LastMessage)

	completedTasks, err := store.ListXnetMigrationTasksByStatus(ctx, types.XnetMigrationTaskStatusCompleted)
	require.Nil(t, err)
	require.Len(t, completedTasks, 1)
	require.Equal(t, "Task 2 completed", completedTasks[0].LastMessage)

	pendingTasks, err := store.ListXnetMigrationTasksByStatus(ctx, types.XnetMigrationTaskStatusPending)
	require.Nil(t, err)
	require.Len(t, pendingTasks, 0)

	// Test with non-existent repo ID
	noTasks, err := store.ListXnetMigrationTasksByRepoID(ctx, 999)
	require.Nil(t, err)
	require.Len(t, noTasks, 0)

	// Test with non-existent task ID
	_, err = store.GetXnetMigrationTaskByID(ctx, 999)
	require.NotNil(t, err)
}
