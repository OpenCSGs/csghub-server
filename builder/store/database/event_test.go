package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestEventStore_Save(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewEventStoreWithDB(db)
	err := store.Save(ctx, database.Event{
		Module: "m1",
	})
	require.Nil(t, err)
	event := &database.Event{}
	err = db.Core.NewSelect().Model(event).Where("module=?", "m1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "m1", event.Module)

	err = store.BatchSave(ctx, []database.Event{
		{Module: "m2"},
		{Module: "m3"},
	})
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(event).Where("module=?", "m2").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "m2", event.Module)
	err = db.Core.NewSelect().Model(event).Where("module=?", "m3").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "m3", event.Module)

}
