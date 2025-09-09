package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAccountPresentStore_AddPresent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: "foo",
		Balance:  5,
	}).Exec(ctx)
	require.Nil(t, err)

	err = store.AddPresent(ctx, database.AccountPresent{
		UserUUID: "foo",
		Value:    123,
	}, database.AccountStatement{
		UserUUID: "foo",
		Value:    123,
	})
	require.Nil(t, err)

	present := &database.AccountPresent{}
	stat := &database.AccountStatement{}
	auser := &database.AccountUser{}

	err = db.Core.NewSelect().Model(present).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(stat).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(auser).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, float64(123), present.Value)
	require.Equal(t, float64(123), stat.Value)
	require.Equal(t, float64(128), auser.Balance)
}

func TestAccountPresentStore_FindPresentByUserIDAndScene(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ps := []database.AccountPresent{
		{UserUUID: "foo", ActivityID: 1, Value: 1},
		{UserUUID: "foo", ActivityID: 2, Value: 2},
		{UserUUID: "bar", ActivityID: 1, Value: 3},
		{UserUUID: "foo", ActivityID: 3, Value: 4},
	}

	for _, p := range ps {
		_, err := db.Core.NewInsert().Model(&p).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountPresentStoreWithDB(db)
	p, err := store.FindPresentByUserIDAndScene(ctx, "foo", 1)
	require.Nil(t, err)
	require.Equal(t, float64(1), p.Value)

}
