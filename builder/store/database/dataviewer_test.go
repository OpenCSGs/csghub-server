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

func TestDataviewerStore_GetLastSuccessfulJobByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dv := database.NewDataViewerStoreWithDB(db)

	// Create jobs with different statuses
	jobPending := database.DataviewerJob{
		RepoID:     1,
		Status:     0, // WorkflowPending
		WorkflowID: "pending-workflow",
		CardData:   "pending-data",
	}
	jobRunning := database.DataviewerJob{
		RepoID:     1,
		Status:     1, // WorkflowRunning
		WorkflowID: "running-workflow",
		CardData:   "running-data",
	}
	jobDone := database.DataviewerJob{
		RepoID:     1,
		Status:     2, // WorkflowDone
		WorkflowID: "done-workflow",
		CardData:   "done-data",
	}
	jobFailed := database.DataviewerJob{
		RepoID:     1,
		Status:     3, // WorkflowFailed
		WorkflowID: "failed-workflow",
		CardData:   "failed-data",
	}

	err := dv.CreateJob(ctx, jobPending)
	require.Nil(t, err)
	err = dv.CreateJob(ctx, jobRunning)
	require.Nil(t, err)
	err = dv.CreateJob(ctx, jobDone)
	require.Nil(t, err)
	err = dv.CreateJob(ctx, jobFailed)
	require.Nil(t, err)

	// Should return only the last successful job (status=2)
	job, err := dv.GetLastSuccessfulJobByRepoID(ctx, int64(1))
	require.Nil(t, err)
	require.NotNil(t, job)
	require.Equal(t, 2, job.Status)
	require.Equal(t, "done-workflow", job.WorkflowID)
	require.Equal(t, "done-data", job.CardData)
}

func TestDataviewerStore_GetLastSuccessfulJobByRepoID_NoMatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dv := database.NewDataViewerStoreWithDB(db)

	// Only pending and running jobs, no successful one
	jobPending := database.DataviewerJob{
		RepoID:     2,
		Status:     0, // WorkflowPending
		WorkflowID: "pending-workflow-2",
	}
	err := dv.CreateJob(ctx, jobPending)
	require.Nil(t, err)

	job, err := dv.GetLastSuccessfulJobByRepoID(ctx, int64(2))
	require.Nil(t, err)
	require.Nil(t, job) // Should return nil when no matching job
}
