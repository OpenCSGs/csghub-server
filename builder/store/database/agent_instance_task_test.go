package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
