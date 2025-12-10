package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestLfsMetaStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)
	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "foobar",
	})
	require.Nil(t, err)

	obj := &database.LfsMetaObject{}
	err = db.Core.NewSelect().Model(obj).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foobar", obj.Oid)

	obj, err = store.FindByOID(ctx, 123, "foobar")
	require.Nil(t, err)
	require.Equal(t, "foobar", obj.Oid)

	objs, err := store.FindByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(objs))
	require.Equal(t, "foobar", objs[0].Oid)

	objs, err = store.FindByRepoID(ctx, 987)
	require.Nil(t, err)
	require.Equal(t, 0, len(objs))

	// update
	_, err = store.UpdateOrCreate(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "foobar",
		Size:         999,
	})
	require.Nil(t, err)
	obj, err = store.FindByOID(ctx, 123, "foobar")
	require.Nil(t, err)
	require.Equal(t, 999, int(obj.Size))

	// create
	_, err = store.UpdateOrCreate(ctx, database.LfsMetaObject{
		RepositoryID: 456,
		Oid:          "bar",
		Size:         998,
	})
	require.Nil(t, err)
	obj, err = store.FindByOID(ctx, 456, "bar")
	require.Nil(t, err)
	require.Equal(t, 998, int(obj.Size))

	err = store.BulkUpdateOrCreate(ctx, int64(456), []database.LfsMetaObject{
		{RepositoryID: 456, Oid: "foobar", Size: 1},
		{RepositoryID: 456, Oid: "bar", Size: 2},
		{RepositoryID: 456, Oid: "barfoo", Size: 3},
	})
	require.Nil(t, err)

	obj, err = store.FindByOID(ctx, 456, "foobar")
	require.Nil(t, err)
	require.Equal(t, 1, int(obj.Size))
	obj, err = store.FindByOID(ctx, 456, "bar")
	require.Nil(t, err)
	require.Equal(t, 2, int(obj.Size))
	obj, err = store.FindByOID(ctx, 456, "barfoo")
	require.Nil(t, err)
	require.Equal(t, 3, int(obj.Size))

	err = store.RemoveByOid(ctx, "foobar", 456)
	require.Nil(t, err)
	_, err = store.FindByOID(ctx, 456, "foobar")
	require.NotNil(t, err)

}

func TestLfsMetaStore_UpdateXnetUsed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)

	// Create a test LFS object
	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "test-oid-123",
		Size:         1024,
		XnetUsed:     false,
	})
	require.Nil(t, err)

	// Verify initial state
	obj, err := store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, "test-oid-123", obj.Oid)
	require.Equal(t, false, obj.XnetUsed)

	// Update XnetUsed to true
	err = store.UpdateXnetUsed(ctx, 123, "test-oid-123", true)
	require.Nil(t, err)

	// Verify update
	obj, err = store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, true, obj.XnetUsed)

	// Update XnetUsed back to false
	err = store.UpdateXnetUsed(ctx, 123, "test-oid-123", false)
	require.Nil(t, err)

	// Verify update
	obj, err = store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, false, obj.XnetUsed)

	// Test updating non-existent object (should not error but affect 0 rows)
	err = store.UpdateXnetUsed(ctx, 999, "non-existent-oid", true)
	require.Nil(t, err)
}
