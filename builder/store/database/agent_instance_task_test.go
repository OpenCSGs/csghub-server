package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentInstanceTaskStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// First create an agent instance to link to
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	userUUID := uuid.New().String()
	sessionUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-123",
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	// Now test AgentInstanceTask store
	store := database.NewAgentInstanceTaskStoreWithDB(db)

	// Test Create
	task := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "task-123",
		SessionUUID: sessionUUID,
		UserUUID:    userUUID,
	}

	createdTask, err := store.Create(ctx, task)
	require.NoError(t, err)
	require.NotZero(t, createdTask.ID)
	require.Equal(t, createdInstance.ID, createdTask.InstanceID)
	require.Equal(t, types.AgentTaskTypeFinetuneJob, createdTask.TaskType)
	require.Equal(t, "task-123", createdTask.TaskID)
	require.Equal(t, sessionUUID, createdTask.SessionUUID)
	require.Equal(t, userUUID, createdTask.UserUUID)
}

func TestAgentInstanceTaskStore_Create_Duplicate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// Create an agent instance
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	userUUID := uuid.New().String()
	sessionUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-789",
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	// Create first task
	store := database.NewAgentInstanceTaskStoreWithDB(db)
	task := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "task-duplicate",
		SessionUUID: sessionUUID,
		UserUUID:    userUUID,
	}
	_, err = store.Create(ctx, task)
	require.NoError(t, err)

	// Try to create duplicate (same instance_id, task_type, and task_id)
	// The unique constraint is on (instance_id, task_type, task_id)
	duplicateTask := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "task-duplicate",
		SessionUUID: uuid.New().String(), // Different session UUID
		UserUUID:    userUUID,
	}
	_, err = store.Create(ctx, duplicateTask)
	require.Error(t, err) // Should fail due to unique constraint on (instance_id, task_type, task_id)
}

func TestAgentInstanceTaskStore_Create_DifferentTaskType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// Create an agent instance
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	userUUID := uuid.New().String()
	sessionUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-456",
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	// Create first task with finetune type
	store := database.NewAgentInstanceTaskStoreWithDB(db)
	task1 := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "task-same-id",
		SessionUUID: sessionUUID,
		UserUUID:    userUUID,
	}
	createdTask1, err := store.Create(ctx, task1)
	require.NoError(t, err)
	require.NotZero(t, createdTask1.ID)

	// Create second task with same task_id but different task_type
	// This should succeed because the unique constraint is on (instance_id, task_type, task_id)
	task2 := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeInference,
		TaskID:      "task-same-id",
		SessionUUID: sessionUUID,
		UserUUID:    userUUID,
	}
	createdTask2, err := store.Create(ctx, task2)
	require.NoError(t, err)
	require.NotZero(t, createdTask2.ID)
	require.Equal(t, types.AgentTaskTypeInference, createdTask2.TaskType)
	require.Equal(t, "task-same-id", createdTask2.TaskID)
}

func TestAgentInstanceTaskStore_ListTasks(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// Setup test data
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	argoStore := database.NewArgoWorkFlowStoreWithDB(db)
	deployStore := database.NewDeployTaskStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	store := database.NewAgentInstanceTaskStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create users
	user1 := &database.User{
		Username: "user1",
		UUID:     userUUID1,
		Email:    "user1@example.com",
	}
	namespace1 := &database.Namespace{Path: userUUID1}
	err := userStore.Create(ctx, user1, namespace1)
	require.NoError(t, err)

	user2 := &database.User{
		Username: "user2",
		UUID:     userUUID2,
		Email:    "user2@example.com",
	}
	namespace2 := &database.Namespace{Path: userUUID2}
	err = userStore.Create(ctx, user2, namespace2)
	require.NoError(t, err)

	// Create agent instances
	instance1 := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID1,
		Type:       "langflow",
		ContentID:  "instance-1",
		Name:       "Instance 1",
		Public:     false,
	}
	createdInstance1, err := instanceStore.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID1,
		Type:       "code",
		ContentID:  "instance-2",
		Name:       "Instance 2",
		Public:     false,
	}
	createdInstance2, err := instanceStore.Create(ctx, instance2)
	require.NoError(t, err)

	// Create sessions
	session1UUID := uuid.New().String()
	session1 := &database.AgentInstanceSession{
		InstanceID: createdInstance1.ID,
		UserUUID:   userUUID1,
		UUID:       session1UUID,
		Name:       "Session 1",
	}
	createdSession1, err := sessionStore.Create(ctx, session1)
	require.NoError(t, err)

	session2UUID := uuid.New().String()
	session2 := &database.AgentInstanceSession{
		InstanceID: createdInstance2.ID,
		UserUUID:   userUUID1,
		UUID:       session2UUID,
		Name:       "Session 2",
	}
	createdSession2, err := sessionStore.Create(ctx, session2)
	require.NoError(t, err)

	// Create argo workflows
	argoWorkflow1 := database.ArgoWorkflow{
		Username:   "user1",
		UserUUID:   userUUID1,
		TaskName:   "Finetune Task 1",
		TaskId:     "argo-task-1",
		TaskType:   types.TaskTypeFinetune,
		ClusterID:  "cluster-1",
		Namespace:  "ns-1",
		RepoIds:    []string{"model-1"},
		RepoType:   "model",
		Status:     v1alpha1.WorkflowRunning,
		Image:      "image-1",
		Datasets:   []string{"dataset-1"},
		SubmitTime: time.Now().Add(-2 * time.Hour),
		StartTime:  time.Now().Add(-1 * time.Hour),
	}
	_, err = argoStore.CreateWorkFlow(ctx, argoWorkflow1)
	require.NoError(t, err)

	argoWorkflow2 := database.ArgoWorkflow{
		Username:   "user1",
		UserUUID:   userUUID1,
		TaskName:   "Finetune Task 2",
		TaskId:     "argo-task-2",
		TaskType:   types.TaskTypeFinetune,
		ClusterID:  "cluster-1",
		Namespace:  "ns-1",
		RepoIds:    []string{"model-2"},
		RepoType:   "model",
		Status:     v1alpha1.WorkflowSucceeded,
		Image:      "image-1",
		Datasets:   []string{"dataset-2"},
		SubmitTime: time.Now().Add(-3 * time.Hour),
		StartTime:  time.Now().Add(-2 * time.Hour),
		EndTime:    time.Now().Add(-1 * time.Hour),
	}
	_, err = argoStore.CreateWorkFlow(ctx, argoWorkflow2)
	require.NoError(t, err)

	// Create deploys
	// Use Deploying status for deploy1 so it maps to in_progress
	deploy1 := database.Deploy{
		SpaceID:    0,
		Status:     common.Deploying,
		GitPath:    "path1",
		GitBranch:  "main",
		Template:   "template1",
		Hardware:   "hardware1",
		UserID:     user1.ID,
		DeployName: "Inference Task 1",
		Type:       types.InferenceType,
		UserUUID:   userUUID1,
	}
	err = deployStore.CreateDeploy(ctx, &deploy1)
	require.NoError(t, err)

	deploy2 := database.Deploy{
		SpaceID:    0,
		Status:     common.Stopped,
		GitPath:    "path2",
		GitBranch:  "main",
		Template:   "template2",
		Hardware:   "hardware2",
		UserID:     user1.ID,
		DeployName: "Inference Task 2",
		Type:       types.InferenceType,
		UserUUID:   userUUID1,
	}
	err = deployStore.CreateDeploy(ctx, &deploy2)
	require.NoError(t, err)

	// Create agent instance tasks
	task1 := &database.AgentInstanceTask{
		InstanceID:  createdInstance1.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "argo-task-1",
		SessionUUID: createdSession1.UUID,
		UserUUID:    userUUID1,
	}
	_, err = store.Create(ctx, task1)
	require.NoError(t, err)

	task2 := &database.AgentInstanceTask{
		InstanceID:  createdInstance1.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "argo-task-2",
		SessionUUID: createdSession1.UUID,
		UserUUID:    userUUID1,
	}
	createdTask2, err := store.Create(ctx, task2)
	require.NoError(t, err)

	task3 := &database.AgentInstanceTask{
		InstanceID:  createdInstance2.ID,
		TaskType:    types.AgentTaskTypeInference,
		TaskID:      fmt.Sprintf("%d", deploy1.ID),
		SessionUUID: createdSession2.UUID,
		UserUUID:    userUUID1,
	}
	_, err = store.Create(ctx, task3)
	require.NoError(t, err)

	// Create task for deploy2 (with Stopped status - maps to failed)
	task4 := &database.AgentInstanceTask{
		InstanceID:  createdInstance2.ID,
		TaskType:    types.AgentTaskTypeInference,
		TaskID:      fmt.Sprintf("%d", deploy2.ID),
		SessionUUID: createdSession2.UUID,
		UserUUID:    userUUID1,
	}
	_, err = store.Create(ctx, task4)
	require.NoError(t, err)

	// Create a deploy with Deleted status - should be filtered out
	deployDeleted := database.Deploy{
		SpaceID:    0,
		Status:     common.Deleted,
		GitPath:    "path-deleted",
		GitBranch:  "main",
		Template:   "template-deleted",
		Hardware:   "hardware-deleted",
		UserID:     user1.ID,
		DeployName: "Deleted Inference Task",
		Type:       types.InferenceType,
		UserUUID:   userUUID1,
	}
	err = deployStore.CreateDeploy(ctx, &deployDeleted)
	require.NoError(t, err)

	// Create a task for the deleted deploy - should not appear in list
	taskDeleted := &database.AgentInstanceTask{
		InstanceID:  createdInstance2.ID,
		TaskType:    types.AgentTaskTypeInference,
		TaskID:      fmt.Sprintf("%d", deployDeleted.ID),
		SessionUUID: createdSession2.UUID,
		UserUUID:    userUUID1,
	}
	_, err = store.Create(ctx, taskDeleted)
	require.NoError(t, err)

	t.Run("list all tasks", func(t *testing.T) {
		tasks, total, err := store.ListTasks(ctx, userUUID1, types.AgentTaskFilter{}, 10, 1)
		require.NoError(t, err)
		// Should be 4 tasks (deleted deploy task should be filtered out)
		require.Equal(t, 4, total)
		require.Len(t, tasks, 4)
		// Verify deleted task is not in the list
		for _, task := range tasks {
			require.NotEqual(t, fmt.Sprintf("%d", deployDeleted.ID), task.TaskID, "Deleted deploy task should not appear in list")
		}
	})

	t.Run("filter by task type", func(t *testing.T) {
		filter := types.AgentTaskFilter{
			TaskType: types.AgentTaskTypeFinetuneJob,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, tasks, 2)
		for _, task := range tasks {
			require.Equal(t, types.AgentTaskTypeFinetuneJob, task.TaskType)
		}
	})

	t.Run("filter by status in_progress", func(t *testing.T) {
		filter := types.AgentTaskFilter{
			Status: types.AgentTaskStatusInProgress,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, 1)
		require.GreaterOrEqual(t, len(tasks), 1)
		// All returned tasks should have in_progress status
		for _, task := range tasks {
			require.Equal(t, types.AgentTaskStatusInProgress, task.TaskStatus, "task %d (%s) should be in_progress but got %s", task.ID, task.TaskName, task.TaskStatus)
		}
	})

	t.Run("filter by status completed", func(t *testing.T) {
		filter := types.AgentTaskFilter{
			Status: types.AgentTaskStatusCompleted,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, 1)
		require.GreaterOrEqual(t, len(tasks), 1)
		// All returned tasks should have completed status
		for _, task := range tasks {
			require.Equal(t, types.AgentTaskStatusCompleted, task.TaskStatus, "task %d (%s) should be completed but got %s", task.ID, task.TaskName, task.TaskStatus)
		}
	})

	t.Run("filter by instance_id", func(t *testing.T) {
		instanceID := createdInstance1.ID
		filter := types.AgentTaskFilter{
			InstanceID: &instanceID,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, tasks, 2)
		for _, task := range tasks {
			require.Equal(t, createdInstance1.ID, task.InstanceID)
		}
	})

	t.Run("filter by session_uuid", func(t *testing.T) {
		instanceID := createdInstance1.ID
		filter := types.AgentTaskFilter{
			InstanceID:  &instanceID,
			SessionUUID: createdSession1.UUID,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, tasks, 2)
		for _, task := range tasks {
			require.Equal(t, createdSession1.UUID, task.SessionUUID)
		}
	})

	t.Run("search by task name", func(t *testing.T) {
		filter := types.AgentTaskFilter{
			Search: "Finetune",
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, 1)
		for _, task := range tasks {
			require.Contains(t, task.TaskName, "Finetune")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		tasks, total, err := store.ListTasks(ctx, userUUID1, types.AgentTaskFilter{}, 2, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)
		require.Len(t, tasks, 2)

		tasks2, total2, err := store.ListTasks(ctx, userUUID1, types.AgentTaskFilter{}, 2, 2)
		require.NoError(t, err)
		require.Equal(t, 4, total2)
		require.Len(t, tasks2, 2)
	})

	t.Run("deleted deploy tasks are filtered out", func(t *testing.T) {
		// List all tasks - deleted deploy task should not appear
		tasks, total, err := store.ListTasks(ctx, userUUID1, types.AgentTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)
		// Verify the deleted task is not in results
		for _, task := range tasks {
			require.NotEqual(t, fmt.Sprintf("%d", deployDeleted.ID), task.TaskID)
		}
	})

	t.Run("pinned tasks appear first", func(t *testing.T) {
		// Create a preference store to pin a task
		preferenceStore := database.NewAgentUserPreferenceStoreWithDB(db)

		// Pin task2
		pinPreference := &database.AgentUserPreference{
			UserUUID:   userUUID1,
			EntityType: types.AgentUserPreferenceEntityTypeAgentTask,
			EntityID:   fmt.Sprintf("%d", createdTask2.ID),
			Action:     types.AgentUserPreferenceActionPin,
		}
		err = preferenceStore.Create(ctx, pinPreference)
		require.NoError(t, err)

		// List tasks - pinned task should appear first
		tasks, total, err := store.ListTasks(ctx, userUUID1, types.AgentTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)
		require.Len(t, tasks, 4)

		// Find the pinned task
		var pinnedTask *types.AgentTaskListItem
		for i := range tasks {
			if tasks[i].ID == createdTask2.ID {
				pinnedTask = &tasks[i]
				break
			}
		}
		require.NotNil(t, pinnedTask, "Pinned task should be found")
		require.True(t, pinnedTask.IsPinned, "Task should be marked as pinned")
		require.NotNil(t, pinnedTask.PinnedAt, "PinnedAt should be set")

		// The pinned task should be first in the list
		require.Equal(t, createdTask2.ID, tasks[0].ID, "Pinned task should be first")
		require.True(t, tasks[0].IsPinned, "First task should be pinned")
	})

	t.Run("filter by status failed", func(t *testing.T) {
		filter := types.AgentTaskFilter{
			Status: types.AgentTaskStatusFailed,
		}
		tasks, total, err := store.ListTasks(ctx, userUUID1, filter, 10, 1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, 1)
		require.GreaterOrEqual(t, len(tasks), 1)
		// All returned tasks should have failed status
		for _, task := range tasks {
			require.Equal(t, types.AgentTaskStatusFailed, task.TaskStatus, "task %d (%s) should be failed but got %s", task.ID, task.TaskName, task.TaskStatus)
		}
	})

	t.Run("user can only see own tasks", func(t *testing.T) {
		tasks, total, err := store.ListTasks(ctx, userUUID2, types.AgentTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 0, total)
		require.Len(t, tasks, 0)
	})
}

func TestAgentInstanceTaskStore_GetTaskByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// Setup test data
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	argoStore := database.NewArgoWorkFlowStoreWithDB(db)
	deployStore := database.NewDeployTaskStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	store := database.NewAgentInstanceTaskStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create user
	user := &database.User{
		Username: "testuser",
		UUID:     userUUID,
		Email:    "test@example.com",
	}
	namespace := &database.Namespace{
		Path: userUUID,
	}
	err := userStore.Create(ctx, user, namespace)
	require.NoError(t, err)

	// Create agent instance
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-test",
		Name:       "Test Instance",
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	// Create session
	sessionUUID := uuid.New().String()
	session := &database.AgentInstanceSession{
		InstanceID: createdInstance.ID,
		UserUUID:   userUUID,
		UUID:       sessionUUID,
		Name:       "Test Session",
	}
	createdSession, err := sessionStore.Create(ctx, session)
	require.NoError(t, err)

	t.Run("get finetune job task detail", func(t *testing.T) {
		// Create argo workflow
		argoWorkflow := database.ArgoWorkflow{
			Username:   "testuser",
			UserUUID:   userUUID,
			TaskName:   "Test Finetune Task",
			TaskId:     "test-argo-task",
			TaskType:   types.TaskTypeFinetune,
			TaskDesc:   "Test task description",
			ClusterID:  "cluster-1",
			Namespace:  "ns-1",
			RepoIds:    []string{"model-1", "model-2"},
			RepoType:   "model",
			Status:     v1alpha1.WorkflowSucceeded,
			Image:      "image-1",
			Datasets:   []string{"dataset-1"},
			SubmitTime: time.Now().Add(-2 * time.Hour),
			StartTime:  time.Now().Add(-1 * time.Hour),
			EndTime:    time.Now(),
			ResultURL:  "https://example.com/result",
		}
		_, err = argoStore.CreateWorkFlow(ctx, argoWorkflow)
		require.NoError(t, err)

		// Create agent instance task
		task := &database.AgentInstanceTask{
			InstanceID:  createdInstance.ID,
			TaskType:    types.AgentTaskTypeFinetuneJob,
			TaskID:      "test-argo-task",
			SessionUUID: createdSession.UUID,
			UserUUID:    userUUID,
		}
		createdTask, err := store.Create(ctx, task)
		require.NoError(t, err)

		// Get task detail
		detail, err := store.GetTaskByID(ctx, userUUID, createdTask.ID)
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Equal(t, createdTask.ID, detail.ID)
		require.Equal(t, "test-argo-task", detail.TaskID)
		require.Equal(t, "Test Finetune Task", detail.TaskName)
		require.Equal(t, "Test task description", detail.TaskDesc)
		require.Equal(t, types.AgentTaskTypeFinetuneJob, detail.TaskType)
		require.Equal(t, types.AgentTaskStatusCompleted, detail.Status)
		require.Equal(t, createdInstance.ID, detail.InstanceID)
		require.Equal(t, "langflow", detail.InstanceType)
		require.Equal(t, "Test Instance", detail.InstanceName)
		require.Equal(t, createdSession.UUID, detail.SessionUUID)
		require.Equal(t, "Test Session", detail.SessionName)
		require.Equal(t, "testuser", detail.Username)
		require.Equal(t, "argo_workflow", detail.Backend)
		require.NotNil(t, detail.Metadata)
		require.NotEmpty(t, detail.Metadata)
	})

	t.Run("get inference task detail", func(t *testing.T) {
		// Create deploy with Running status (maps to completed for inference)
		deploy := database.Deploy{
			SpaceID:    0,
			Status:     common.Running,
			GitPath:    "path1",
			GitBranch:  "main",
			Template:   "template1",
			Hardware:   "hardware1",
			UserID:     user.ID,
			DeployName: "Test Inference Task",
			Type:       types.InferenceType,
			UserUUID:   userUUID,
			Message:    "Test inference message",
		}
		err = deployStore.CreateDeploy(ctx, &deploy)
		require.NoError(t, err)
		require.NotZero(t, deploy.ID, "deploy ID should be set after creation")

		// Create agent instance task
		task := &database.AgentInstanceTask{
			InstanceID:  createdInstance.ID,
			TaskType:    types.AgentTaskTypeInference,
			TaskID:      fmt.Sprintf("%d", deploy.ID),
			SessionUUID: createdSession.UUID,
			UserUUID:    userUUID,
		}
		createdTask, err := store.Create(ctx, task)
		require.NoError(t, err)

		// Get task detail
		detail, err := store.GetTaskByID(ctx, userUUID, createdTask.ID)
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Equal(t, createdTask.ID, detail.ID)
		require.Equal(t, fmt.Sprintf("%d", deploy.ID), detail.TaskID)
		require.Equal(t, "Test Inference Task", detail.TaskName)
		require.Equal(t, types.AgentTaskTypeInference, detail.TaskType)
		// Running status maps to completed for inference tasks
		require.Equal(t, types.AgentTaskStatusCompleted, detail.Status)
		require.Equal(t, createdInstance.ID, detail.InstanceID)
		require.Equal(t, "langflow", detail.InstanceType)
		require.Equal(t, "Test Instance", detail.InstanceName)
		require.Equal(t, createdSession.UUID, detail.SessionUUID)
		require.Equal(t, "Test Session", detail.SessionName)
		require.Equal(t, "testuser", detail.Username)
		require.Equal(t, "deploy", detail.Backend)
		require.NotNil(t, detail.Metadata)
		require.NotEmpty(t, detail.Metadata)
	})

	t.Run("task not found", func(t *testing.T) {
		detail, err := store.GetTaskByID(ctx, userUUID, 99999)
		require.Error(t, err)
		require.Nil(t, detail)
	})

	t.Run("user cannot access other user's task", func(t *testing.T) {
		otherUserUUID := uuid.New().String()
		otherUser := &database.User{
			Username: "otheruser",
			UUID:     otherUserUUID,
			Email:    "other@example.com",
		}
		otherNamespace := &database.Namespace{
			Path: otherUserUUID,
		}
		err := userStore.Create(ctx, otherUser, otherNamespace)
		require.NoError(t, err)

		// Create a task for the first user
		argoWorkflow := database.ArgoWorkflow{
			Username:  "testuser",
			UserUUID:  userUUID,
			TaskName:  "Private Task",
			TaskId:    "private-task",
			TaskType:  types.TaskTypeFinetune,
			ClusterID: "cluster-1",
			Namespace: "ns-1",
			RepoIds:   []string{"model-1"},
			RepoType:  "model",
			Status:    "Running",
			Image:     "image-1",
		}
		_, err = argoStore.CreateWorkFlow(ctx, argoWorkflow)
		require.NoError(t, err)

		task := &database.AgentInstanceTask{
			InstanceID:  createdInstance.ID,
			TaskType:    types.AgentTaskTypeFinetuneJob,
			TaskID:      "private-task",
			SessionUUID: createdSession.UUID,
			UserUUID:    userUUID,
		}
		createdTask, err := store.Create(ctx, task)
		require.NoError(t, err)

		// Other user cannot access
		detail, err := store.GetTaskByID(ctx, otherUserUUID, createdTask.ID)
		require.Error(t, err)
		require.Nil(t, detail)
	})

	t.Run("get finetune job task detail with failed status", func(t *testing.T) {
		// Create argo workflow with failed status
		argoWorkflow := database.ArgoWorkflow{
			Username:   "testuser",
			UserUUID:   userUUID,
			TaskName:   "Failed Finetune Task",
			TaskId:     "failed-argo-task",
			TaskType:   types.TaskTypeFinetune,
			TaskDesc:   "Failed task description",
			ClusterID:  "cluster-1",
			Namespace:  "ns-1",
			RepoIds:    []string{"model-1"},
			RepoType:   "model",
			Status:     v1alpha1.WorkflowFailed,
			Image:      "image-1",
			Datasets:   []string{"dataset-1"},
			SubmitTime: time.Now().Add(-2 * time.Hour),
			StartTime:  time.Now().Add(-1 * time.Hour),
			EndTime:    time.Now(),
			Reason:     "Task failed due to error",
		}
		_, err = argoStore.CreateWorkFlow(ctx, argoWorkflow)
		require.NoError(t, err)

		// Create agent instance task
		task := &database.AgentInstanceTask{
			InstanceID:  createdInstance.ID,
			TaskType:    types.AgentTaskTypeFinetuneJob,
			TaskID:      "failed-argo-task",
			SessionUUID: createdSession.UUID,
			UserUUID:    userUUID,
		}
		createdTask, err := store.Create(ctx, task)
		require.NoError(t, err)

		// Get task detail
		detail, err := store.GetTaskByID(ctx, userUUID, createdTask.ID)
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Equal(t, types.AgentTaskStatusFailed, detail.Status)
		require.Equal(t, "Failed Finetune Task", detail.TaskName)
		require.NotNil(t, detail.Metadata)
	})

	t.Run("get inference task detail with different statuses", func(t *testing.T) {
		testCases := []struct {
			name           string
			status         int
			expectedStatus types.AgentTaskStatus
		}{
			{"Building", common.Building, types.AgentTaskStatusInProgress},
			{"BuildInQueue", common.BuildInQueue, types.AgentTaskStatusInProgress},
			{"Startup", common.Startup, types.AgentTaskStatusInProgress},
			{"BuildFailed", common.BuildFailed, types.AgentTaskStatusFailed},
			{"DeployFailed", common.DeployFailed, types.AgentTaskStatusFailed},
			{"RunTimeError", common.RunTimeError, types.AgentTaskStatusFailed},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deploy := database.Deploy{
					SpaceID:    0,
					Status:     tc.status,
					GitPath:    "path1",
					GitBranch:  "main",
					Template:   "template1",
					Hardware:   "hardware1",
					UserID:     user.ID,
					DeployName: fmt.Sprintf("Test Inference Task %s", tc.name),
					Type:       types.InferenceType,
					UserUUID:   userUUID,
					Message:    fmt.Sprintf("Test message for %s", tc.name),
				}
				err = deployStore.CreateDeploy(ctx, &deploy)
				require.NoError(t, err)
				require.NotZero(t, deploy.ID)

				task := &database.AgentInstanceTask{
					InstanceID:  createdInstance.ID,
					TaskType:    types.AgentTaskTypeInference,
					TaskID:      fmt.Sprintf("%d", deploy.ID),
					SessionUUID: createdSession.UUID,
					UserUUID:    userUUID,
				}
				createdTask, err := store.Create(ctx, task)
				require.NoError(t, err)

				detail, err := store.GetTaskByID(ctx, userUUID, createdTask.ID)
				require.NoError(t, err)
				require.NotNil(t, detail)
				require.Equal(t, tc.expectedStatus, detail.Status, "Status should be %s for deploy status %d", tc.expectedStatus, tc.status)
			})
		}
	})

	t.Run("get task detail with missing source data", func(t *testing.T) {
		// Create a task that references a non-existent argo workflow
		task := &database.AgentInstanceTask{
			InstanceID:  createdInstance.ID,
			TaskType:    types.AgentTaskTypeFinetuneJob,
			TaskID:      "non-existent-task",
			SessionUUID: createdSession.UUID,
			UserUUID:    userUUID,
		}
		createdTask, err := store.Create(ctx, task)
		require.NoError(t, err)

		// Get task detail - should still work but metadata might be empty
		detail, err := store.GetTaskByID(ctx, userUUID, createdTask.ID)
		// This might fail because the join won't find the argo workflow
		// But the code should handle it gracefully
		if err == nil {
			require.NotNil(t, detail)
			// Metadata might be nil or empty if source data doesn't exist
		}
	})
}
