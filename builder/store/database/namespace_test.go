package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestNamespaceStore_All(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewNamespaceStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.Namespace{
		Path: "foo/bar",
	}).Exec(ctx)
	require.Nil(t, err)

	exist, err := store.Exists(ctx, "foo/bar")
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.Exists(ctx, "foo/bar2")
	require.Nil(t, err)
	require.False(t, exist)

	ns, err := store.FindByPath(ctx, "foo/bar")
	require.Nil(t, err)
	require.Equal(t, "foo/bar", ns.Path)
	_, err = store.FindByPath(ctx, "foo/bar2")
	require.NotNil(t, err)

}
