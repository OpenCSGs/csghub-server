package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorTaskStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	m, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	var mt database.MirrorTask
	err = db.Core.NewSelect().Model(&mt).Where("id = ?", m.ID).Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, int64(1), mt.ID)
	require.Equal(t, int64(1), mt.MirrorID)
	require.Equal(t, types.MirrorQueued, mt.Status)
	require.Equal(t, types.LowMirrorPriority, mt.Priority)

	m1, err := store.Update(ctx, database.MirrorTask{
		ID:       1,
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	var mt1 database.MirrorTask
	err = db.Core.NewSelect().Model(&mt1).Where("id = ?", m.ID).Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, types.MirrorQueued, m1.Status)
	require.Equal(t, types.LowMirrorPriority, m1.Priority)
	require.Equal(t, types.MirrorQueued, mt1.Status)
	require.Equal(t, types.LowMirrorPriority, mt1.Priority)

	m2, err := store.FindByMirrorID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, int64(1), m2.ID)
	require.Equal(t, int64(1), m2.MirrorID)
	require.Equal(t, types.MirrorQueued, m2.Status)
	require.Equal(t, types.LowMirrorPriority, m2.Priority)

	err = store.Delete(ctx, 1)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(&mt).Where("id = ?", m.ID).Scan(ctx)
	require.NotNil(t, err)
	require.Equal(t, "sql: no rows in result set", err.Error())
}

func TestMirrorTaskStore_GetHighestPriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	_, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	mt, err := store.GetHighestPriorityByTaskStatus(ctx, []types.MirrorTaskStatus{})
	require.Nil(t, err)
	require.Equal(t, int64(1), mt.MirrorID)
	require.Equal(t, types.MirrorRepoSyncStart, mt.Status)
	require.Equal(t, types.HighMirrorPriority, mt.Priority)
}

func TestMirrorTaskStore_SetMirrorCurrentTaskID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)
	mstore := database.NewMirrorStoreWithDB(db)

	mt, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	_, err = mstore.Create(ctx, &database.Mirror{
		Interval:       "1",
		SourceUrl:      "test",
		RepositoryID:   1,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	err = store.SetMirrorCurrentTaskID(ctx, mt)
	require.Nil(t, err)

	var m database.Mirror

	err = db.Operator.Core.NewSelect().Model(&m).Where("id = ?", 1).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, mt.ID, m.CurrentTaskID)
}
