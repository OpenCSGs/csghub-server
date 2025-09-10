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
		TableName: "repositories",
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
		TableName: "repositories",
		Value:     "{id: 1}",
	})
	require.Nil(t, err)

	pds, err := store.FindByTableName(ctx, "repositories")
	require.Nil(t, err)
	require.Len(t, pds, 1)
	require.Equal(t, "repositories", pds[0].TableName)
	require.Equal(t, "{id: 1}", pds[0].Value)
}
