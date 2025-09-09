package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSpaceResourceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	_, err := store.Create(ctx, database.SpaceResource{
		Name:      "r1",
		ClusterID: "c1",
	})
	require.Nil(t, err)
	sr := &database.SpaceResource{}
	err = db.Core.NewSelect().Model(sr).Where("name=?", "r1").Scan(ctx, sr)
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	sr, err = store.FindByID(ctx, sr.ID)
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	sr, err = store.FindByName(ctx, "r1")
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	srs, err := store.FindAll(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(srs))
	require.Equal(t, "c1", srs[0].ClusterID)

	srs, _, err = store.Index(ctx, "c1", 50, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(srs))
	require.Equal(t, "c1", srs[0].ClusterID)

	sr.Name = "r2"
	_, err = store.Update(ctx, *sr)
	require.Nil(t, err)
	sr, err = store.FindByID(ctx, sr.ID)
	require.Nil(t, err)
	require.Equal(t, "r2", sr.Name)

	err = store.Delete(ctx, *sr)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, sr.ID)
	require.NotNil(t, err)

}
