package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestDatasetApplicationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create
	app, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:        1,
		ApplicantID:      100,
		Action:           types.DatasetApplicationActionInitial,
		Price:            9.99,
		RelatedDatasetID: 2,
		Status:           types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)
	require.NotZero(t, app.ID)
	require.Equal(t, int64(1), app.DatasetID)
	require.Equal(t, types.DatasetApplicationStatusPending, app.Status)

	// FindByID
	found, err := store.FindByID(ctx, app.ID)
	require.Nil(t, err)
	require.Equal(t, app.ID, found.ID)
	require.Equal(t, types.DatasetApplicationActionInitial, found.Action)

	// Update
	found.Status = types.DatasetApplicationStatusApproved
	err = store.Update(ctx, *found)
	require.Nil(t, err)

	found, err = store.FindByID(ctx, app.ID)
	require.Nil(t, err)
	require.Equal(t, types.DatasetApplicationStatusApproved, found.Status)
}

func TestDatasetApplicationStore_FindByDatasetID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create apps for dataset 1
	_, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusApproved,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionEdit,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	// Create app for dataset 2
	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   2,
		ApplicantID: 200,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	apps, err := store.FindByDatasetID(ctx, 1)
	require.Nil(t, err)
	require.Len(t, apps, 2)

	apps, err = store.FindByDatasetID(ctx, 2)
	require.Nil(t, err)
	require.Len(t, apps, 1)

	apps, err = store.FindByDatasetID(ctx, 999)
	require.Nil(t, err)
	require.Len(t, apps, 0)
}

func TestDatasetApplicationStore_FindPendingByDatasetID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create a pending app
	_, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	pending, err := store.FindPendingByDatasetID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, types.DatasetApplicationStatusPending, pending.Status)

	// No pending app for dataset 2
	_, err = store.FindPendingByDatasetID(ctx, 2)
	require.NotNil(t, err)

	// Create an approved app and verify it's not found as pending
	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   3,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusApproved,
	})
	require.Nil(t, err)

	_, err = store.FindPendingByDatasetID(ctx, 3)
	require.NotNil(t, err)
}

func TestDatasetApplicationStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	_, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   2,
		ApplicantID: 200,
		Action:      types.DatasetApplicationActionEdit,
		Status:      types.DatasetApplicationStatusApproved,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   3,
		ApplicantID: 300,
		Action:      types.DatasetApplicationActionDelist,
		Status:      types.DatasetApplicationStatusRejected,
	})
	require.Nil(t, err)

	// List all
	apps, total, err := store.List(ctx, "", "", 10, 1)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, apps, 3)

	// Filter by status
	apps, total, err = store.List(ctx, string(types.DatasetApplicationStatusPending), "", 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, apps, 1)
	require.Equal(t, types.DatasetApplicationStatusPending, apps[0].Status)

	// Pagination
	apps, total, err = store.List(ctx, "", "", 2, 1)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, apps, 2)
}

func TestDatasetApplicationStore_DuplicatePendingRejected(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create first pending application
	_, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 100,
		Action:      types.DatasetApplicationActionInitial,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	// Try to create another pending application for the same dataset - should fail
	_, err = store.Create(ctx, database.DatasetApplication{
		DatasetID:   1,
		ApplicantID: 200,
		Action:      types.DatasetApplicationActionEdit,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestDatasetApplicationStore_ConcurrentCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	const goroutines = 10
	results := make(chan error, goroutines)

	for i := range goroutines {
		go func(idx int) {
			_, err := store.Create(ctx, database.DatasetApplication{
				DatasetID:   1,
				ApplicantID: int64(100 + idx),
				Action:      types.DatasetApplicationActionInitial,
				Status:      types.DatasetApplicationStatusPending,
			})
			results <- err
		}(i)
	}

	successCount := 0
	for range goroutines {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	// Exactly one should succeed due to the unique partial index
	require.Equal(t, 1, successCount)
}
