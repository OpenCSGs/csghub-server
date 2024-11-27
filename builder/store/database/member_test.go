package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestMemberStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMemberStoreWithDB(db)

	err := store.Add(ctx, 123, 456, "foo")
	require.Nil(t, err)
	mem := &database.Member{}
	err = db.Core.NewSelect().Model(mem).Where("user_id=?", 456).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", mem.Role)

	mem, err = store.Find(ctx, 123, 456)
	require.Nil(t, err)
	require.Equal(t, "foo", mem.Role)

	ms, err := store.UserMembers(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, "foo", ms[0].Role)

	ms, count, err := store.OrganizationMembers(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, 1, count)
	require.Equal(t, "foo", ms[0].Role)

	err = store.Delete(ctx, 123, 456, "foo")
	require.Nil(t, err)
	_, err = store.Find(ctx, 123, 456)
	require.NotNil(t, err)

}
