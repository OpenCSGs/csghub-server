package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployTaskStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)

	err := store.CreateDeploy(ctx, &database.Deploy{
		DeployName: "dp1", SvcName: "s1",
		RepoID:  123,
		UserID:  456,
		SpaceID: 321,
		Type:    types.ServerlessType,
	})
	require.Nil(t, err)
	dp := &database.Deploy{}
	err = db.Core.NewSelect().Model(dp).Where("deploy_name=?", "dp1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dp.DeployName, "dp1")

	dp, err = store.GetDeployByID(ctx, dp.ID)
	require.Nil(t, err)
	require.Equal(t, dp.DeployName, "dp1")

	dp.DeployName = "foo"
	err = store.UpdateDeploy(ctx, dp)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dp).Where("deploy_name=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dp.DeployName, "foo")

	dp, err = store.GetDeployBySvcName(ctx, "s1")
	require.Nil(t, err)
	require.Equal(t, dp.DeployName, "foo")

	err = store.StopDeploy(ctx, types.ModelRepo, 123, 456, dp.ID)
	require.Nil(t, err)
	dp, err = store.GetDeployByID(ctx, dp.ID)
	require.Nil(t, err)
	require.Equal(t, dp.Status, common.Stopped)

	err = store.CreateDeploy(ctx, &database.Deploy{
		DeployName: "dp2", SvcName: "s2",
		RepoID:  123,
		UserID:  456,
		SpaceID: 321,
	})
	require.Nil(t, err)
	dp, err = store.GetLatestDeployBySpaceID(ctx, 321)
	require.Nil(t, err)
	require.Equal(t, dp.SvcName, "s2")

	dp, err = store.GetServerlessDeployByRepID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, dp.SvcName, "s1")
	dps, total, err := store.ListServerless(ctx, types.DeployReq{
		DeployType: types.ServerlessType,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, dps[0].SvcName, "s1")

	err = store.DeleteDeploy(ctx, types.ModelRepo, 123, 456, dp.ID)
	require.Nil(t, err)
	dp, err = store.GetDeployByID(ctx, dp.ID)
	require.Nil(t, err)
	require.Equal(t, dp.Status, common.Deleted)

}
func TestDeployTaskStore_DeployTaskCRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)

	err := store.CreateDeployTask(ctx, &database.DeployTask{
		DeployID: 1,
		Message:  "foo",
	})
	require.Nil(t, err)
	dp := &database.DeployTask{}
	err = db.Core.NewSelect().Model(dp).Where("deploy_id=?", 1).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dp.Message, "foo")

	dp, err = store.GetDeployTask(ctx, dp.ID)
	require.Nil(t, err)
	require.Equal(t, dp.Message, "foo")

	dp.Message = "bar"
	err = store.UpdateDeployTask(ctx, dp)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dp).Where("deploy_id=?", 1).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dp.Message, "bar")

	tasks, err := store.GetDeployTasksOfDeploy(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, "bar", tasks[0].Message)

}

func TestDeployTaskStore_GetNewTaskAfter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)

	err := store.CreateDeploy(ctx, &database.Deploy{SvcName: "svc"})
	require.Nil(t, err)
	dp, err := store.GetDeployBySvcName(ctx, "svc")
	require.Nil(t, err)

	tasks := []*database.DeployTask{
		{TaskType: 0, Status: 0, Message: "t1"},
		{TaskType: 0, Status: 1, Message: "t2"},
		{TaskType: 0, Status: 2, Message: "t3"},
		{TaskType: 0, Status: 3, Message: "t4"},
		{TaskType: 1, Status: 0, Message: "t5"},
		{TaskType: 1, Status: 1, Message: "t6"},
		{TaskType: 1, Status: 2, Message: "t7"},
		{TaskType: 1, Status: 3, Message: "t8"},
	}

	for _, tk := range tasks {
		tk.DeployID = dp.ID
		err = store.CreateDeployTask(ctx, tk)
		require.Nil(t, err)
	}

	for _, c := range []struct {
		current  int64
		expected string
		err      bool
	}{
		{0, "t1", false},
		{tasks[0].ID, "t2", false},
		{tasks[1].ID, "t5", false},
		{tasks[2].ID, "t5", false},
		{tasks[3].ID, "t5", false},
		{tasks[4].ID, "t6", false},
		{tasks[5].ID, "t8", false},
		{tasks[6].ID, "t8", false},
		{tasks[7].ID, "t8", true},
	} {
		tk, err := store.GetNewTaskAfter(ctx, c.current)
		if c.err {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
			require.Equal(t, c.expected, tk.Message)
		}
	}

	first, err := store.GetNewTaskFirst(ctx)
	require.Nil(t, err)
	require.Equal(t, "t1", first.Message)

}

func TestDeployTaskStore_UpdateInTx(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)
	err := store.CreateDeploy(ctx, &database.Deploy{
		SvcName:   "svc",
		GitPath:   "test",
		GitBranch: "test",
		Endpoint:  "test",
		Env:       "test",
	})
	require.Nil(t, err)
	dp, err := store.GetDeployBySvcName(ctx, "svc")
	require.Nil(t, err)

	tasks := []*database.DeployTask{
		{TaskType: 3, Status: 1, Message: "t1"},
		{TaskType: 3, Status: 2, Message: "t2"},
	}

	for _, tk := range tasks {
		tk.DeployID = dp.ID
		err = store.CreateDeployTask(ctx, tk)
		require.Nil(t, err)
	}
	tasks[0].Message = "t1new"
	tasks[0].TaskType = 1
	tasks[0].Status = 3
	tasks[1].Message = "t2new"
	tasks[1].TaskType = 1
	tasks[1].Status = 3

	dp.GitPath = "foo/bar"
	dp.GitBranch = "new"
	dp.Endpoint = "eee"
	dp.Env = "env"
	err = store.UpdateInTx(ctx, []string{"git_path", "git_branch"}, []string{"message"}, dp, tasks...)
	require.Nil(t, err)

	dp, err = store.GetDeployBySvcName(ctx, "svc")
	require.Nil(t, err)
	require.Equal(t, "foo/bar", dp.GitPath)
	require.Equal(t, "new", dp.GitBranch)
	require.Equal(t, "test", dp.Endpoint)
	require.Equal(t, "test", dp.Env)

	tasks, err = store.GetDeployTasksOfDeploy(ctx, dp.ID)
	require.Nil(t, err)
	messages := []string{}
	types := []int{}
	for _, t := range tasks {
		messages = append(messages, t.Message)
		types = append(types, t.TaskType)
	}
	require.ElementsMatch(t, []string{"t1new", "t2new"}, messages)
	require.ElementsMatch(t, []int{3, 3}, types)

}
