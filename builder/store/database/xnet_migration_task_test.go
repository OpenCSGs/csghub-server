package database_test

import (
	"context"
	"fmt"
	"sync"
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
	_, err := store.CreateXnetMigrationTask(ctx, 123, "Initial task")
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
	_, err = store.CreateXnetMigrationTask(ctx, 456, "Task 2")
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

func TestXnetMigrationTaskStore_EnsureActive(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: fmt.Sprintf("xnet-owner/xnet-ensure-%s", t.Name()),
	}))
	require.NoError(t, err)
	store := database.NewXnetMigrationTaskStoreWithDB(db)

	first, err := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusPending, first.Status)
	require.NoError(t, store.UpdateXnetMigrationTask(ctx, first.ID, "exhausted", types.XnetMigrationTaskStatusFailed))

	const callers = 2
	results := make(chan *database.XnetMigrationTask, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, ensureErr := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
			results <- task
			errs <- ensureErr
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for ensureErr := range errs {
		require.NoError(t, ensureErr)
	}
	var replacementID int64
	for task := range results {
		require.NotNil(t, task)
		if replacementID == 0 {
			replacementID = task.ID
		}
		require.Equal(t, replacementID, task.ID)
	}
	require.NotEqual(t, first.ID, replacementID)

	loadedFirst, err := store.GetXnetMigrationTaskByID(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusFailed, loadedFirst.Status)
	loadedReplacement, err := store.GetXnetMigrationTaskByID(ctx, replacementID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusPending, loadedReplacement.Status)
	loadedRepo, err := database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, replacementID, loadedRepo.CurrentXnetMigrationTaskID)
}

func TestXnetMigrationTaskStore_EnsureActiveConcurrentInitialCreation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: fmt.Sprintf("xnet-owner/xnet-ensure-initial-%s", t.Name()),
	}))
	require.NoError(t, err)
	store := database.NewXnetMigrationTaskStoreWithDB(db)

	const callers = 2
	results := make(chan *database.XnetMigrationTask, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, ensureErr := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
			results <- task
			errs <- ensureErr
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for ensureErr := range errs {
		require.NoError(t, ensureErr)
	}

	var taskID int64
	for task := range results {
		require.NotNil(t, task)
		if taskID == 0 {
			taskID = task.ID
		}
		require.Equal(t, taskID, task.ID)
	}
	tasks, err := store.ListXnetMigrationTasksByRepoID(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, taskID, tasks[0].ID)
	loadedRepo, err := database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, taskID, loadedRepo.CurrentXnetMigrationTaskID)
}

func TestXnetMigrationTaskStore_UpdateIfNonTerminal(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewXnetMigrationTaskStoreWithDB(db)

	for _, test := range []struct {
		name            string
		initialStatus   types.XnetMigrationTaskStatus
		requestedStatus types.XnetMigrationTaskStatus
		expectedStatus  types.XnetMigrationTaskStatus
		expectedMsg     string
	}{
		{name: "pending transitions to running", initialStatus: types.XnetMigrationTaskStatusPending, requestedStatus: types.XnetMigrationTaskStatusRunning, expectedStatus: types.XnetMigrationTaskStatusRunning, expectedMsg: "late update"},
		{name: "completed is not overwritten by failed", initialStatus: types.XnetMigrationTaskStatusCompleted, requestedStatus: types.XnetMigrationTaskStatusFailed, expectedStatus: types.XnetMigrationTaskStatusCompleted, expectedMsg: "terminal"},
		{name: "failed is not overwritten by completed", initialStatus: types.XnetMigrationTaskStatusFailed, requestedStatus: types.XnetMigrationTaskStatusCompleted, expectedStatus: types.XnetMigrationTaskStatusFailed, expectedMsg: "terminal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := store.CreateXnetMigrationTask(ctx, 123, "terminal")
			require.NoError(t, err)
			if test.initialStatus != types.XnetMigrationTaskStatusPending {
				require.NoError(t, store.UpdateXnetMigrationTask(ctx, task.ID, "terminal", test.initialStatus))
			}

			require.NoError(t, store.UpdateXnetMigrationTaskIfNonTerminal(ctx, task.ID, "late update", test.requestedStatus))

			updated, err := store.GetXnetMigrationTaskByID(ctx, task.ID)
			require.NoError(t, err)
			require.Equal(t, test.expectedStatus, updated.Status)
			require.Equal(t, test.expectedMsg, updated.LastMessage)
		})
	}
}

func TestXnetMigrationTaskStore_CompleteCurrentConvergesAfterConcurrentFailure(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: fmt.Sprintf("xnet-owner/xnet-complete-current-%s", t.Name()),
	}))
	require.NoError(t, err)
	store := database.NewXnetMigrationTaskStoreWithDB(db)
	task, err := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
	require.NoError(t, err)
	require.NoError(t, store.UpdateXnetMigrationTask(ctx, task.ID, "running", types.XnetMigrationTaskStatusRunning))

	// Simulate sync observing exhaustion after the result handler already read
	// the task as running, but before it persists the completed state.
	staleTask, err := store.GetXnetMigrationTaskByID(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusRunning, staleTask.Status)
	require.NoError(t, store.UpdateXnetMigrationTaskIfNonTerminal(ctx, task.ID, "exhausted", types.XnetMigrationTaskStatusFailed))

	require.NoError(t, store.CompleteCurrentXnetMigrationTask(ctx, repo.ID, "late success"))

	completed, err := store.GetXnetMigrationTaskByID(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusCompleted, completed.Status)
	require.Equal(t, "late success", completed.LastMessage)
	updatedRepo, err := database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.True(t, updatedRepo.XnetEnabled)
}

func TestXnetMigrationTaskStore_CompleteCurrentCompletesReplacementTask(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db)).CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: fmt.Sprintf("xnet-owner/xnet-complete-attached-%s", t.Name()),
	}))
	require.NoError(t, err)
	store := database.NewXnetMigrationTaskStoreWithDB(db)
	oldTask, err := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
	require.NoError(t, err)
	require.NoError(t, store.UpdateXnetMigrationTask(ctx, oldTask.ID, "failed", types.XnetMigrationTaskStatusFailed))
	currentTask, err := store.EnsureActiveXnetMigrationTask(ctx, repo.ID)
	require.NoError(t, err)

	require.NoError(t, store.CompleteCurrentXnetMigrationTask(ctx, repo.ID, "done"))

	oldTask, err = store.GetXnetMigrationTaskByID(ctx, oldTask.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusFailed, oldTask.Status)
	currentTask, err = store.GetXnetMigrationTaskByID(ctx, currentTask.ID)
	require.NoError(t, err)
	require.Equal(t, types.XnetMigrationTaskStatusCompleted, currentTask.Status)
	require.Equal(t, "done", currentTask.LastMessage)
	updatedRepo, err := database.NewRepoStoreWithDB(db).FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, currentTask.ID, updatedRepo.CurrentXnetMigrationTaskID)
	require.True(t, updatedRepo.XnetEnabled)
}
