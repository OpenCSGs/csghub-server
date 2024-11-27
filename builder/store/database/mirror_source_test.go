package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestMirrorSourceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorSourceStoreWithDB(db)
	_, err := store.Create(ctx, &database.MirrorSource{
		SourceName: "foo",
	})
	require.Nil(t, err)

	mi := &database.MirrorSource{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.SourceName)

	mi, err = store.Get(ctx, mi.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.SourceName)

	mi, err = store.FindByName(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", mi.SourceName)

	mi.SourceName = "bar"
	err = store.Update(ctx, mi)
	require.Nil(t, err)
	mi = &database.MirrorSource{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", mi.SourceName)

	mis, err := store.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(mis))
	require.Equal(t, "bar", mis[0].SourceName)

	err = store.Delete(ctx, mi)
	require.Nil(t, err)
	_, err = store.Get(ctx, mi.ID)
	require.NotNil(t, err)

}
