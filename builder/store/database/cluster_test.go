package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestClusterStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewClusterInfoStoreWithDB(db)

	err := store.Add(ctx, "foo", "bar")
	require.Nil(t, err)

	cfg := &database.ClusterInfo{}
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", cfg.Region)

	// already exist, do nothing
	err = store.Add(ctx, "foo", "bar2")
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", cfg.Region)

	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = store.Update(ctx, database.ClusterInfo{
		ClusterID:     cfg.ClusterID,
		ClusterConfig: "foo",
		Region:        "bar3",
	})
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(cfg).Where("cluster_config=?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar3", cfg.Region)

	dbCluster := &database.ClusterInfo{}
	err = db.Core.NewSelect().Model(dbCluster).Limit(1).Scan(ctx)
	require.Nil(t, err)
	info, err := store.ByClusterID(ctx, dbCluster.ClusterID)
	require.Nil(t, err)
	require.Equal(t, "bar3", info.Region)

	infos, err := store.List(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(infos))
	require.Equal(t, "bar3", infos[0].Region)

}
