package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSpaceSDKStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceSdkStoreWithDB(db)

	_, err := store.Create(ctx, database.SpaceSdk{
		Name:    "r1",
		Version: "v1",
	})
	require.Nil(t, err)
	ss := &database.SpaceSdk{}
	err = db.Core.NewSelect().Model(ss).Where("name=?", "r1").Scan(ctx, ss)
	require.Nil(t, err)
	require.Equal(t, "v1", ss.Version)

	ss, err = store.FindByID(ctx, ss.ID)
	require.Nil(t, err)
	require.Equal(t, "v1", ss.Version)

	sss, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(sss))
	require.Equal(t, "v1", sss[0].Version)

	ss.Name = "r2"
	_, err = store.Update(ctx, *ss)
	require.Nil(t, err)
	ss, err = store.FindByID(ctx, ss.ID)
	require.Nil(t, err)
	require.Equal(t, "r2", ss.Name)

	err = store.Delete(ctx, *ss)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, ss.ID)
	require.NotNil(t, err)

}
