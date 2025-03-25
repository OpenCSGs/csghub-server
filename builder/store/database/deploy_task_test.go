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

func TestDeployTaskStore_GetRunningInferenceAndFinetuneByUserID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)
	deploys := []database.Deploy{
		{UserID: 123, Type: 1, Status: common.Running, DeployName: "d1"},
		{UserID: 123, Type: 0, Status: common.Running, DeployName: "d2"},
		{UserID: 123, Type: 2, Status: common.Running, DeployName: "d3"},
		{UserID: 123, Type: 3, Status: common.Running, DeployName: "d4"},
		{UserID: 123, Type: 1, Status: common.Stopped, DeployName: "d5"},
		{UserID: 456, Type: 1, Status: common.Running, DeployName: "d6"},
	}

	for _, dp := range deploys {
		err := store.CreateDeploy(ctx, &dp)
		require.Nil(t, err)
	}

	dps, err := store.GetRunningInferenceAndFinetuneByUserID(ctx, 123)
	require.Nil(t, err)
	names := []string{}
	for _, dp := range dps {
		names = append(names, dp.DeployName)
	}
	require.ElementsMatch(t, []string{"d1", "d3"}, names)

}

func TestDeployTaskStore_RunningVisibleToUser(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskStoreWithDB(db)

	// user 1 's public dedicated inference
	deploy1 := database.Deploy{
		ID:          1,
		DeployName:  "deploy1",
		SvcName:     "svc1",
		RepoID:      1,
		UserID:      1,
		Type:        1,
		SecureLevel: 1,
		Status:      common.Running,
	}
	// user 2 's public fintune
	deploy2 := database.Deploy{
		ID:          2,
		DeployName:  "deploy2",
		SvcName:     "svc2",
		RepoID:      2,
		UserID:      2,
		Type:        2,
		SecureLevel: 1,
		Status:      common.Running,
	}
	// user 1 's private dedicated inference
	deploy3 := database.Deploy{
		ID:          3,
		DeployName:  "deploy3",
		SvcName:     "svc3",
		RepoID:      3,
		UserID:      1,
		Type:        1,
		SecureLevel: 2, //private
		Status:      common.Running,
	}
	// user 2 's public dedicated inference
	deploy4 := database.Deploy{
		ID:          4,
		DeployName:  "deploy4",
		SvcName:     "svc4",
		RepoID:      4,
		UserID:      2,
		Type:        1,
		SecureLevel: 1,
		Status:      common.Running,
	}
	// user 3 's serverless inference
	deploy5 := database.Deploy{
		ID:          5,
		DeployName:  "deploy5",
		SvcName:     "svc5",
		RepoID:      5,
		UserID:      3,
		Type:        3,
		SecureLevel: 1,
		Status:      common.Running,
	}
	// user 3 's serverless inference not running
	deploy6 := database.Deploy{
		ID:          6,
		DeployName:  "deploy6",
		SvcName:     "svc6",
		RepoID:      6,
		UserID:      3,
		Type:        3,
		SecureLevel: 1,
		Status:      common.Stopped,
	}

	// Insert test data into the database
	err := store.CreateDeploy(ctx, &deploy1)
	require.Nil(t, err)
	err = store.CreateDeploy(ctx, &deploy2)
	require.Nil(t, err)
	err = store.CreateDeploy(ctx, &deploy3)
	require.Nil(t, err)
	err = store.CreateDeploy(ctx, &deploy4)
	require.Nil(t, err)
	err = store.CreateDeploy(ctx, &deploy5)
	require.Nil(t, err)
	err = store.CreateDeploy(ctx, &deploy6)
	require.Nil(t, err)

	// Test RunningVisibleToUser with user ID 1
	deploys, err := store.RunningVisibleToUser(ctx, 1)
	require.Nil(t, err)
	require.Len(t, deploys, 4)
	require.Equal(t, deploy1.ID, deploys[0].ID)
	require.Equal(t, deploy3.ID, deploys[1].ID)
	require.Equal(t, deploy4.ID, deploys[2].ID)
	require.Equal(t, deploy5.ID, deploys[3].ID)

	// Test RunningVisibleToUser with user ID 2
	deploys, err = store.RunningVisibleToUser(ctx, 2)
	require.Nil(t, err)
	require.Len(t, deploys, 3)
	require.Equal(t, deploy1.ID, deploys[0].ID)
	require.Equal(t, deploy4.ID, deploys[1].ID)
	require.Equal(t, deploy5.ID, deploys[2].ID)

	// Test RunningVisibleToUser with user ID 3
	deploys, err = store.RunningVisibleToUser(ctx, 3)
	require.Nil(t, err)
	require.Len(t, deploys, 3)
	require.Equal(t, deploy1.ID, deploys[0].ID)
	require.Equal(t, deploy4.ID, deploys[1].ID)
	require.Equal(t, deploy5.ID, deploys[2].ID)
}
