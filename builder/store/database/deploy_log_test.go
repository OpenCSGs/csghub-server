package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestDeployLogStore_DeployLogs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDeployTaskLogWithDB(db)

	log := database.DeployLog{
		ClusterID:        "clsid",
		SvcName:          "svc",
		PodName:          "pod1",
		UserContainerLog: "test log1",
	}

	res, err := store.UpdateDeployLogs(ctx, log)
	require.Nil(t, err)
	require.Equal(t, "svc", res.SvcName)
	require.Equal(t, "pod1", res.PodName)

	res, err = store.GetDeployLogs(ctx, log)
	require.Nil(t, err)
	require.Equal(t, "svc", res.SvcName)
	require.Equal(t, "pod1", res.PodName)
	require.Equal(t, "test log1", res.UserContainerLog)
}
