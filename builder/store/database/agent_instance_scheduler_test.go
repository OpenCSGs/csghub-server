package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentInstanceSchedulerStore_CountByUserUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	schedulerStore := database.NewAgentInstanceSchedulerStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create agent instances for both users (use unique content_id per user to satisfy unique constraint)
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

	// Count with no schedulers (should return 0)
	count, err := schedulerStore.CountByUserUUID(ctx, userUUID1)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Create active and paused schedulers for user1
	s1 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID1,
		InstanceID:     createdInstance1.ID,
		Name:           "s1",
		Prompt:         "p1",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	_, err = schedulerStore.Create(ctx, s1)
	require.NoError(t, err)

	s2 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID1,
		InstanceID:     createdInstance1.ID,
		Name:           "s2",
		Prompt:         "p2",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 2 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusPaused,
	}
	_, err = schedulerStore.Create(ctx, s2)
	require.NoError(t, err)

	// Count should be 2 (active + paused, excluding finished)
	count, err = schedulerStore.CountByUserUUID(ctx, userUUID1)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	// Create finished scheduler for user1
	s3 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID1,
		InstanceID:     createdInstance1.ID,
		Name:           "s3",
		Prompt:         "p3",
		ScheduleType:   types.AgentScheduleTypeOnce,
		CronExpression: "0 1 1 1 *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusFinished,
	}
	_, err = schedulerStore.Create(ctx, s3)
	require.NoError(t, err)

	// Count should still be 2 (finished excluded)
	count, err = schedulerStore.CountByUserUUID(ctx, userUUID1)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	// Create scheduler for user2
	s4 := &database.AgentInstanceScheduler{
		UserUUID:       userUUID2,
		InstanceID:     createdInstance2.ID,
		Name:           "s4",
		Prompt:         "p4",
		ScheduleType:   types.AgentScheduleTypeDaily,
		CronExpression: "0 1 * * *",
		StartDate:      baseTime,
		StartTime:      baseTime,
		Status:         types.AgentSchedulerStatusActive,
	}
	_, err = schedulerStore.Create(ctx, s4)
	require.NoError(t, err)

	count, err = schedulerStore.CountByUserUUID(ctx, userUUID2)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Count with non-existent user (should return 0)
	nonExistentUser := uuid.New().String()
	count, err = schedulerStore.CountByUserUUID(ctx, nonExistentUser)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
