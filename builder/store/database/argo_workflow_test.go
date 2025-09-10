package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
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
		SubmitTime: dt.Add(-5 * time.Hour),
	})
	require.Nil(t, err)
	flows, total, err := store.FindByUsername(ctx, "user", 10, 1)
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
