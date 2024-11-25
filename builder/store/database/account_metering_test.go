package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountMeteringStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	am := database.AccountMetering{
		UserUUID:     "foo",
		Value:        12.34,
		ValueType:    1,
		ResourceName: "abc",
	}
	err := store.Create(ctx, am)
	require.Nil(t, err)
	amn := &database.AccountMetering{}
	err = db.Core.NewSelect().Model(amn).Where("user_uuid = ?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", amn.UserUUID)
	require.Equal(t, 12.34, amn.Value)
	require.Equal(t, 1, amn.ValueType)
	require.Equal(t, "abc", amn.ResourceName)
}

func TestAccountMeteringStore_ListByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)
	ams := []database.AccountMetering{
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r1", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r2", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(-2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r3", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r4", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r5", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r6", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(-6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r7", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 12.34, ValueType: 1,
			ResourceName: "r8", Scene: types.SceneSpace, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r9", Scene: types.SceneMultiSync, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r10", Scene: types.SceneSpace, CustomerID: "barz",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
	}

	for _, am := range ams {
		err := store.Create(ctx, am)
		require.Nil(t, err)
	}

	ams, total, err := store.ListByUserIDAndTime(ctx, types.ACCT_STATEMENTS_REQ{
		UserUUID:     "foo",
		Scene:        11,
		InstanceName: "bar",
		StartTime:    dt.Add(-5 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(5 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 5, total)
	names := []string{}
	for _, am := range ams {
		names = append(names, am.ResourceName)
	}
	require.Equal(t, []string{"r5", "r4", "r3", "r2", "r1"}, names)
}

func TestAccountMeteringStore_ListAllByUserUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	ams := []database.AccountMetering{
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r1", Scene: types.SceneSpace, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r2", Scene: types.SceneSpace, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r3", Scene: types.SceneSpace, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 12.34, ValueType: 1,
			ResourceName: "r4", Scene: types.SceneSpace, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
	}

	for _, am := range ams {
		err := store.Create(ctx, am)
		require.Nil(t, err)
	}
	data, err := store.ListAllByUserUUID(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, 3, len(data))
}
