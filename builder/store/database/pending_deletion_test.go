package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestPendingDeletionCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPendingDeletionStoreWithDB(db)
	err := store.Create(ctx, &database.PendingDeletion{
		TableName: database.PendingDeletionTableNameRepository,
		Value:     "{id: 1}",
	})

	require.Nil(t, err)
}

func TestPendingDeletionFindByTableName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPendingDeletionStoreWithDB(db)
	err := store.Create(ctx, &database.PendingDeletion{
		TableName: database.PendingDeletionTableNameRepository,
		Value:     "{id: 1}",
	})
	require.Nil(t, err)

	pds, err := store.FindByTableNameWithBatch(ctx, database.PendingDeletionTableNameRepository, 5, 0)
	require.Nil(t, err)
	require.Len(t, pds, 1)
	require.Equal(t, database.PendingDeletionTableNameRepository, pds[0].TableName)
	require.Equal(t, "{id: 1}", pds[0].Value)
}

func TestPendingDeletionDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPendingDeletionStoreWithDB(db)
	err := store.Create(ctx, &database.PendingDeletion{
		TableName: database.PendingDeletionTableNameRepository,
		Value:     "{id: 1}",
	})

	require.Nil(t, err)
	var pd database.PendingDeletion
	err = db.Core.NewSelect().Model(&pd).Where("table_name = ?", database.PendingDeletionTableNameRepository).Scan(ctx, &pd)
	require.Nil(t, err)
	require.Equal(t, database.PendingDeletionTableNameRepository, pd.TableName)
	require.Equal(t, "{id: 1}", pd.Value)

	err = store.Delete(ctx, &pd)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(&pd).Where("table_name = ?", database.PendingDeletionTableNameRepository).Scan(ctx, &pd)
	require.NotNil(t, err)
}
