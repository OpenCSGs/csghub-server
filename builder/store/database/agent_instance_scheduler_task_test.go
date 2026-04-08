package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentInstanceSchedulerTaskStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	schedulerStore := database.NewAgentInstanceSchedulerStoreWithDB(db)
	taskStore := database.NewAgentInstanceSchedulerTaskStoreWithDB(db)

	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID,
		Type:       "code",
		ContentID:  "genius-agent-" + userUUID,
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	baseTime := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	scheduler := &database.AgentInstanceScheduler{
		UserUUID:       userUUID,
		InstanceID:     createdInstance.ID,
		Name:           "test-scheduler",
		Prompt:         "test prompt",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	createdScheduler, err := schedulerStore.Create(ctx, scheduler)
	require.NoError(t, err)

	task := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdScheduler.ID,
		InstanceID:  createdInstance.ID,
		UserUUID:    userUUID,
		Name:        "test-task",
		WorkflowID:  "wf-123",
		SessionUUID: uuid.New().String(),
		Status:      types.AgentSchedulerTaskStatusRunning,
		StartedAt:   baseTime,
	}

	createdTask, err := taskStore.Create(ctx, task)
	require.NoError(t, err)
	require.NotZero(t, createdTask.ID)
	require.Equal(t, createdScheduler.ID, createdTask.SchedulerID)
	require.Equal(t, createdInstance.ID, createdTask.InstanceID)
	require.Equal(t, userUUID, createdTask.UserUUID)
	require.Equal(t, "test-task", createdTask.Name)
	require.Equal(t, "wf-123", createdTask.WorkflowID)
	require.Equal(t, types.AgentSchedulerTaskStatusRunning, createdTask.Status)
}

func TestAgentInstanceSchedulerTaskStore_FindByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	schedulerStore := database.NewAgentInstanceSchedulerStoreWithDB(db)
	taskStore := database.NewAgentInstanceSchedulerTaskStoreWithDB(db)

	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID,
		Type:       "code",
		ContentID:  "genius-agent-" + userUUID,
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	baseTime := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	scheduler := &database.AgentInstanceScheduler{
		UserUUID:       userUUID,
		InstanceID:     createdInstance.ID,
		Name:           "test-scheduler",
		Prompt:         "test prompt",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	createdScheduler, err := schedulerStore.Create(ctx, scheduler)
	require.NoError(t, err)

	task := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdScheduler.ID,
		InstanceID:  createdInstance.ID,
		UserUUID:    userUUID,
		Name:        "find-me",
		Status:      types.AgentSchedulerTaskStatusSuccess,
		StartedAt:   baseTime,
	}
	createdTask, err := taskStore.Create(ctx, task)
	require.NoError(t, err)

	found, err := taskStore.FindByID(ctx, createdTask.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, createdTask.ID, found.ID)
	require.Equal(t, "find-me", found.Name)
	require.Equal(t, types.AgentSchedulerTaskStatusSuccess, found.Status)

	_, err = taskStore.FindByID(ctx, 99999)
	require.Error(t, err)
}

func TestAgentInstanceSchedulerTaskStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	schedulerStore := database.NewAgentInstanceSchedulerStoreWithDB(db)
	taskStore := database.NewAgentInstanceSchedulerTaskStoreWithDB(db)

	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID,
		Type:       "code",
		ContentID:  "genius-agent-" + userUUID,
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)

	baseTime := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	scheduler := &database.AgentInstanceScheduler{
		UserUUID:       userUUID,
		InstanceID:     createdInstance.ID,
		Name:           "test-scheduler",
		Prompt:         "test prompt",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	createdScheduler, err := schedulerStore.Create(ctx, scheduler)
	require.NoError(t, err)

	task := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdScheduler.ID,
		InstanceID:  createdInstance.ID,
		UserUUID:    userUUID,
		Name:        "update-me",
		Status:      types.AgentSchedulerTaskStatusRunning,
		StartedAt:   baseTime,
	}
	createdTask, err := taskStore.Create(ctx, task)
	require.NoError(t, err)

	completedAt := baseTime.Add(time.Hour)
	createdTask.Status = types.AgentSchedulerTaskStatusSuccess
	createdTask.CompletedAt = &completedAt
	createdTask.ErrorMessage = ""

	err = taskStore.Update(ctx, createdTask)
	require.NoError(t, err)

	updated, err := taskStore.FindByID(ctx, createdTask.ID)
	require.NoError(t, err)
	require.Equal(t, types.AgentSchedulerTaskStatusSuccess, updated.Status)
	require.NotNil(t, updated.CompletedAt)
	require.Equal(t, completedAt, *updated.CompletedAt)
}

func TestAgentInstanceSchedulerTaskStore_ListByInstanceID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	schedulerStore := database.NewAgentInstanceSchedulerStoreWithDB(db)
	taskStore := database.NewAgentInstanceSchedulerTaskStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	instance1 := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID1,
		Type:       "code",
		ContentID:  "genius-agent-" + userUUID1,
		Public:     false,
	}
	createdInstance1, err := instanceStore.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID: 2,
		UserUUID:   userUUID2,
		Type:       "code",
		ContentID:  "genius-agent-" + userUUID2,
		Public:     false,
	}
	createdInstance2, err := instanceStore.Create(ctx, instance2)
	require.NoError(t, err)

	baseTime := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)

	s1 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID1,
		InstanceID:     createdInstance1.ID,
		Name:           "scheduler-1",
		Prompt:         "p1",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	createdS1, err := schedulerStore.Create(ctx, s1)
	require.NoError(t, err)

	s2 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID1,
		InstanceID:     createdInstance1.ID,
		Name:           "scheduler-2",
		Prompt:         "p2",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 2 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	createdS2, err := schedulerStore.Create(ctx, s2)
	require.NoError(t, err)

	// Create tasks for instance1
	task1 := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdS1.ID,
		InstanceID:  createdInstance1.ID,
		UserUUID:    userUUID1,
		Name:        "Alpha Task Running",
		Status:      types.AgentSchedulerTaskStatusRunning,
		StartedAt:   baseTime,
	}
	_, err = taskStore.Create(ctx, task1)
	require.NoError(t, err)

	task2 := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdS1.ID,
		InstanceID:  createdInstance1.ID,
		UserUUID:    userUUID1,
		Name:        "Beta Task Success",
		Status:      types.AgentSchedulerTaskStatusSuccess,
		StartedAt:   baseTime,
	}
	_, err = taskStore.Create(ctx, task2)
	require.NoError(t, err)

	task3 := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdS2.ID,
		InstanceID:  createdInstance1.ID,
		UserUUID:    userUUID1,
		Name:        "Gamma Task Failed",
		Status:      types.AgentSchedulerTaskStatusFailed,
		StartedAt:   baseTime,
	}
	_, err = taskStore.Create(ctx, task3)
	require.NoError(t, err)

	// Create task for instance2 (different user)
	task4 := &database.AgentInstanceSchedulerTask{
		SchedulerID: createdS2.ID,
		InstanceID:  createdInstance2.ID,
		UserUUID:    userUUID2,
		Name:        "Delta Task",
		Status:      types.AgentSchedulerTaskStatusSuccess,
		StartedAt:   baseTime,
	}
	_, err = taskStore.Create(ctx, task4)
	require.NoError(t, err)

	t.Run("list all tasks for instance", func(t *testing.T) {
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, types.AgentSchedulerTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Len(t, tasks, 3)
	})

	t.Run("filter by status", func(t *testing.T) {
		filter := types.AgentSchedulerTaskFilter{Status: types.AgentSchedulerTaskStatusRunning}
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Len(t, tasks, 1)
		require.Equal(t, types.AgentSchedulerTaskStatusRunning, tasks[0].Status)
	})

	t.Run("filter by scheduler_id", func(t *testing.T) {
		schedulerID := createdS1.ID
		filter := types.AgentSchedulerTaskFilter{SchedulerID: &schedulerID}
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, tasks, 2)
		for _, task := range tasks {
			require.Equal(t, createdS1.ID, task.SchedulerID)
		}
	})

	t.Run("search by name", func(t *testing.T) {
		filter := types.AgentSchedulerTaskFilter{Search: "Alpha"}
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Len(t, tasks, 1)
		require.Contains(t, tasks[0].Name, "Alpha")
	})

	t.Run("search by name case insensitive", func(t *testing.T) {
		filter := types.AgentSchedulerTaskFilter{Search: "beta"}
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, filter, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Len(t, tasks, 1)
		require.Contains(t, strings.ToLower(tasks[0].Name), "beta")
	})

	t.Run("pagination", func(t *testing.T) {
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, types.AgentSchedulerTaskFilter{}, 2, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Len(t, tasks, 2)

		tasks2, total2, err := taskStore.ListByInstanceID(ctx, userUUID1, createdInstance1.ID, types.AgentSchedulerTaskFilter{}, 2, 2)
		require.NoError(t, err)
		require.Equal(t, 3, total2)
		require.Len(t, tasks2, 1)
	})

	t.Run("user can only see own tasks", func(t *testing.T) {
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID2, createdInstance1.ID, types.AgentSchedulerTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 0, total)
		require.Len(t, tasks, 0)
	})

	t.Run("empty result for non-existent instance", func(t *testing.T) {
		tasks, total, err := taskStore.ListByInstanceID(ctx, userUUID1, 99999, types.AgentSchedulerTaskFilter{}, 10, 1)
		require.NoError(t, err)
		require.Equal(t, 0, total)
		require.Len(t, tasks, 0)
	})
}
