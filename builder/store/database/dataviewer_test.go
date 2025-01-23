package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestDataviewerStore_CreateAndGetViewerByRepoIDAndUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	viewer := database.Dataviewer{
		RepoID:     1,
		RepoPath:   "test/path",
		RepoBranch: "test-branch",
		WorkflowID: "test-workflow",
	}

	dv := database.NewDataViewerStoreWithDB(db)
	err := dv.CreateViewer(ctx, viewer)
	require.Nil(t, err)

	res, err := dv.GetViewerByRepoID(ctx, int64(1))
	require.Nil(t, err)
	require.Equal(t, res.RepoID, int64(1))
	require.Equal(t, res.WorkflowID, "test-workflow")

	viewer.WorkflowID = "updated-workflow"
	res, err = dv.UpdateViewer(ctx, viewer)
	require.Nil(t, err)
	require.Equal(t, res.WorkflowID, "updated-workflow")
}

func TestDataviewerStore_CreateAndGetAndUpdateJob(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	job := database.DataviewerJob{
		RepoID:     1,
		Status:     0,
		WorkflowID: "test-workflow",
	}
	dv := database.NewDataViewerStoreWithDB(db)
	err := dv.CreateJob(ctx, job)
	require.Nil(t, err)

	res, err := dv.GetJob(ctx, "test-workflow")
	require.Nil(t, err)
	require.Equal(t, res.WorkflowID, "test-workflow")

	job.ID = res.ID
	job.CardData = "updated-data"
	res, err = dv.UpdateJob(ctx, job)
	require.Nil(t, err)
	require.Equal(t, res.CardData, "updated-data")
}

func TestDataviewerStore_GetRunningJobsByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	job1 := database.DataviewerJob{
		RepoID:     1,
		Status:     0,
		WorkflowID: "test-workflow1",
	}

	job2 := database.DataviewerJob{
		RepoID:     1,
		Status:     1,
		WorkflowID: "test-workflow2",
	}

	dv := database.NewDataViewerStoreWithDB(db)
	err := dv.CreateJob(ctx, job1)
	require.Nil(t, err)

	err = dv.CreateJob(ctx, job2)
	require.Nil(t, err)

	jobs, err := dv.GetRunningJobsByRepoID(ctx, int64(1))
	require.Nil(t, err)
	require.Equal(t, len(jobs), 2)
}
