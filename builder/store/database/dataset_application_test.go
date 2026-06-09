package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
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

func setupReviewTest(t *testing.T) (*database.DB, context.Context, *database.Dataset, *database.DatasetApplication) {
	t.Helper()
	db := tests.InitTestDB()
	ctx := context.TODO()

	// Create repository and dataset
	repo := &database.Repository{
		Name:    "test-repo",
		Path:    "ns/test-repo",
		GitPath: "ns/test-repo",
	}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.Nil(t, err)

	ds := &database.Dataset{
		RepositoryID: repo.ID,
		Status:       types.DatasetStatusNormal,
	}
	_, err = db.Core.NewInsert().Model(ds).Exec(ctx)
	require.Nil(t, err)
	ds.Repository = repo

	// Create user
	user := &database.User{Username: "applicant", Email: "a@b.com", UUID: "uuid-applicant"}
	_, err = db.Core.NewInsert().Model(user).Exec(ctx)
	require.Nil(t, err)

	// Create application
	store := database.NewDatasetApplicationStoreWithDB(db)
	app, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   ds.ID,
		ApplicantID: user.ID,
		Action:      types.DatasetApplicationActionInitial,
		Price:       9.99,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	return db, ctx, ds, app
}

func TestReviewApplication_Reject(t *testing.T) {
	db, ctx, ds, app := setupReviewTest(t)
	defer db.Close()

	store := database.NewDatasetApplicationStoreWithDB(db)

	result, err := store.ReviewApplication(ctx, app.ID, 999, "not good", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusRejected, nil)
	require.Nil(t, err)
	require.Equal(t, types.DatasetApplicationStatusRejected, result.Status)
	require.Equal(t, int64(999), result.ReviewerID)
	require.Equal(t, "not good", result.ReviewMsg)

	// Dataset should be unchanged
	var refreshedDs database.Dataset
	err = db.Core.NewSelect().Model(&refreshedDs).Where("id = ?", ds.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, ds.Status, refreshedDs.Status)
	require.Equal(t, ds.Price, refreshedDs.Price)
}

func TestReviewApplication_Approve(t *testing.T) {
	db, ctx, ds, app := setupReviewTest(t)
	defer db.Close()

	store := database.NewDatasetApplicationStoreWithDB(db)

	dsUpdate := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 5.00,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}

	result, err := store.ReviewApplication(ctx, app.ID, 888, "approved", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.Nil(t, err)
	require.Equal(t, types.DatasetApplicationStatusApproved, result.Status)
	require.Equal(t, int64(888), result.ReviewerID)

	// Dataset should be updated
	var refreshedDs database.Dataset
	err = db.Core.NewSelect().Model(&refreshedDs).Relation("Repository").Where("dataset.id = ?", ds.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.DatasetStatusListed, refreshedDs.Status)
	require.Equal(t, 5.00, refreshedDs.Price)
	require.Equal(t, types.DatasetTypeCommercial, refreshedDs.DatasetType)
	require.False(t, refreshedDs.Repository.Private)
}

func TestReviewApplication_Approve_Delist(t *testing.T) {
	db, ctx, _, app := setupReviewTest(t)
	defer db.Close()

	store := database.NewDatasetApplicationStoreWithDB(db)

	dsUpdate := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusDelisted,
		Price:                 0,
		DatasetType:           types.DatasetTypeNormal,
		RepositoryPrivate:     true,
	}

	result, err := store.ReviewApplication(ctx, app.ID, 777, "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.Nil(t, err)
	require.Equal(t, types.DatasetApplicationStatusApproved, result.Status)
}

func TestReviewApplication_NonPending(t *testing.T) {
	db, ctx, _, app := setupReviewTest(t)
	defer db.Close()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// First approve the application
	dsUpdate := &database.ReviewDatasetUpdate{ExpectedDatasetStatus: types.DatasetStatusNormal, NewStatus: types.DatasetStatusListed, DatasetType: types.DatasetTypeCommercial}
	_, err := store.ReviewApplication(ctx, app.ID, 999, "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.Nil(t, err)

	// Try to approve again - should fail because status is already approved, not pending
	_, err = store.ReviewApplication(ctx, app.ID, 999, "again", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "status changed")
}

func TestReviewApplication_RelatedDatasetConflict(t *testing.T) {
	db, ctx, ds, app := setupReviewTest(t)
	defer db.Close()

	// Create another dataset that already references current ds as related
	repo2 := &database.Repository{Name: "other", Path: "ns/other", GitPath: "ns/other"}
	_, err := db.Core.NewInsert().Model(repo2).Exec(ctx)
	require.Nil(t, err)
	otherDs := &database.Dataset{RepositoryID: repo2.ID, Status: types.DatasetStatusNormal, RelatedDatasetID: ds.ID}
	_, err = db.Core.NewInsert().Model(otherDs).Exec(ctx)
	require.Nil(t, err)

	// Update app to have a related_dataset_id
	_, err = db.Core.NewUpdate().Model(app).Set("related_dataset_id = ?", otherDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)

	store := database.NewDatasetApplicationStoreWithDB(db)

	dsUpdate := &database.ReviewDatasetUpdate{ExpectedDatasetStatus: types.DatasetStatusNormal, NewStatus: types.DatasetStatusListed, DatasetType: types.DatasetTypeCommercial}
	_, err = store.ReviewApplication(ctx, app.ID, 999, "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrDatasetAlreadyReferenced))
}

func TestReviewApplication_RelatedDatasetConflictReverse(t *testing.T) {
	db, ctx, _, app := setupReviewTest(t)
	defer db.Close()

	// Create a target dataset that will be the contested related dataset
	repo2 := &database.Repository{Name: "target", Path: "ns/target", GitPath: "ns/target"}
	_, err := db.Core.NewInsert().Model(repo2).Exec(ctx)
	require.Nil(t, err)
	targetDs := &database.Dataset{RepositoryID: repo2.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(targetDs).Exec(ctx)
	require.Nil(t, err)

	// Create another dataset that already references the target as its related dataset
	repo3 := &database.Repository{Name: "other", Path: "ns/other", GitPath: "ns/other"}
	_, err = db.Core.NewInsert().Model(repo3).Exec(ctx)
	require.Nil(t, err)
	otherDs := &database.Dataset{RepositoryID: repo3.ID, Status: types.DatasetStatusNormal, RelatedDatasetID: targetDs.ID}
	_, err = db.Core.NewInsert().Model(otherDs).Exec(ctx)
	require.Nil(t, err)

	// Update app to also reference the same target dataset as its related_dataset_id
	_, err = db.Core.NewUpdate().Model(app).Set("related_dataset_id = ?", targetDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)

	store := database.NewDatasetApplicationStoreWithDB(db)

	dsUpdate := &database.ReviewDatasetUpdate{ExpectedDatasetStatus: types.DatasetStatusNormal, NewStatus: types.DatasetStatusListed, DatasetType: types.DatasetTypeCommercial}
	_, err = store.ReviewApplication(ctx, app.ID, 999, "", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrRelatedDatasetAlreadyReferenced))
}

func TestReviewApplication_ConcurrentRelatedDatasetID(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create repos and datasets
	repo1 := &database.Repository{Name: "ds1", Path: "ns/ds1", GitPath: "ns/ds1"}
	_, err := db.Core.NewInsert().Model(repo1).Exec(ctx)
	require.Nil(t, err)
	ds1 := &database.Dataset{RepositoryID: repo1.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(ds1).Exec(ctx)
	require.Nil(t, err)

	repo2 := &database.Repository{Name: "ds2", Path: "ns/ds2", GitPath: "ns/ds2"}
	_, err = db.Core.NewInsert().Model(repo2).Exec(ctx)
	require.Nil(t, err)
	ds2 := &database.Dataset{RepositoryID: repo2.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(ds2).Exec(ctx)
	require.Nil(t, err)

	repoTarget := &database.Repository{Name: "target", Path: "ns/target", GitPath: "ns/target"}
	_, err = db.Core.NewInsert().Model(repoTarget).Exec(ctx)
	require.Nil(t, err)
	targetDs := &database.Dataset{RepositoryID: repoTarget.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(targetDs).Exec(ctx)
	require.Nil(t, err)

	// Create users
	user1 := &database.User{Username: "applicant1", Email: "a@b.com", UUID: "uuid-app1"}
	_, err = db.Core.NewInsert().Model(user1).Exec(ctx)
	require.Nil(t, err)
	user2 := &database.User{Username: "applicant2", Email: "b@b.com", UUID: "uuid-app2"}
	_, err = db.Core.NewInsert().Model(user2).Exec(ctx)
	require.Nil(t, err)

	// Create applications
	app1, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   ds1.ID,
		ApplicantID: user1.ID,
		Action:      types.DatasetApplicationActionInitial,
		Price:       9.99,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)
	app2, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   ds2.ID,
		ApplicantID: user2.ID,
		Action:      types.DatasetApplicationActionInitial,
		Price:       9.99,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	// Set both applications to reference the same target dataset
	_, err = db.Core.NewUpdate().Model(app1).Set("related_dataset_id = ?", targetDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().Model(app2).Set("related_dataset_id = ?", targetDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)

	dsUpdate1 := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 5.00,
		RelatedDatasetID:      targetDs.ID,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}
	dsUpdate2 := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 10.00,
		RelatedDatasetID:      targetDs.ID,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}

	results := make(chan error, 2)

	go func() {
		_, err := store.ReviewApplication(ctx, app1.ID, 999, "approved", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate1)
		results <- err
	}()
	go func() {
		_, err := store.ReviewApplication(ctx, app2.ID, 888, "approved", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate2)
		results <- err
	}()

	successCount := 0
	failCount := 0
	for range 2 {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failCount++
			require.True(t, errors.Is(err, errorx.ErrRelatedDatasetAlreadyReferenced),
				"expected ErrRelatedDatasetAlreadyReferenced, got %v", err)
		}
	}

	require.Equal(t, 1, successCount, "exactly one concurrent review should succeed")
	require.Equal(t, 1, failCount, "exactly one concurrent review should fail")
}

func TestReviewApplication_ConcurrentSameRelatedDatasetID(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDatasetApplicationStoreWithDB(db)

	// Create repos and datasets
	repo1 := &database.Repository{Name: "ds1", Path: "ns/ds1", GitPath: "ns/ds1"}
	_, err := db.Core.NewInsert().Model(repo1).Exec(ctx)
	require.Nil(t, err)
	ds1 := &database.Dataset{RepositoryID: repo1.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(ds1).Exec(ctx)
	require.Nil(t, err)

	repo3 := &database.Repository{Name: "ds3", Path: "ns/ds3", GitPath: "ns/ds3"}
	_, err = db.Core.NewInsert().Model(repo3).Exec(ctx)
	require.Nil(t, err)
	ds3 := &database.Dataset{RepositoryID: repo3.ID, Status: types.DatasetStatusNormal}
	_, err = db.Core.NewInsert().Model(ds3).Exec(ctx)
	require.Nil(t, err)

	repoTarget := &database.Repository{Name: "target2", Path: "ns/target2", GitPath: "ns/target2"}
	_, err = db.Core.NewInsert().Model(repoTarget).Exec(ctx)
	require.Nil(t, err)
	targetDs := &database.Dataset{RepositoryID: repoTarget.ID, Status: types.DatasetStatusListed, RelatedDatasetID: 0}
	_, err = db.Core.NewInsert().Model(targetDs).Exec(ctx)
	require.Nil(t, err)

	// Create users
	user1 := &database.User{Username: "applicant1", Email: "a@b.com", UUID: "uuid-app1"}
	_, err = db.Core.NewInsert().Model(user1).Exec(ctx)
	require.Nil(t, err)
	user3 := &database.User{Username: "applicant3", Email: "c@c.com", UUID: "uuid-app3"}
	_, err = db.Core.NewInsert().Model(user3).Exec(ctx)
	require.Nil(t, err)

	// Create applications
	app1, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   ds1.ID,
		ApplicantID: user1.ID,
		Action:      types.DatasetApplicationActionInitial,
		Price:       9.99,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)
	app3, err := store.Create(ctx, database.DatasetApplication{
		DatasetID:   ds3.ID,
		ApplicantID: user3.ID,
		Action:      types.DatasetApplicationActionInitial,
		Price:       9.99,
		Status:      types.DatasetApplicationStatusPending,
	})
	require.Nil(t, err)

	// Set both applications to reference the same target dataset
	_, err = db.Core.NewUpdate().Model(app1).Set("related_dataset_id = ?", targetDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().Model(app3).Set("related_dataset_id = ?", targetDs.ID).WherePK().Exec(ctx)
	require.Nil(t, err)

	dsUpdate1 := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 5.00,
		RelatedDatasetID:      targetDs.ID,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}
	dsUpdate3 := &database.ReviewDatasetUpdate{
		ExpectedDatasetStatus: types.DatasetStatusNormal,
		NewStatus:             types.DatasetStatusListed,
		Price:                 10.00,
		RelatedDatasetID:      targetDs.ID,
		DatasetType:           types.DatasetTypeCommercial,
		RepositoryPrivate:     false,
	}

	results := make(chan error, 2)

	go func() {
		_, err := store.ReviewApplication(ctx, app1.ID, 999, "approved", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate1)
		results <- err
	}()
	go func() {
		_, err := store.ReviewApplication(ctx, app3.ID, 888, "approved", types.DatasetApplicationStatusPending, types.DatasetApplicationStatusApproved, dsUpdate3)
		results <- err
	}()

	successCount := 0
	failCount := 0
	for range 2 {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failCount++
			require.True(t, errors.Is(err, errorx.ErrRelatedDatasetAlreadyReferenced),
				"expected ErrRelatedDatasetAlreadyReferenced, got %v", err)
		}
	}

	require.Equal(t, 1, successCount, "exactly one concurrent review should succeed")
	require.Equal(t, 1, failCount, "exactly one concurrent review should fail")
}

func TestDatasetApplicationStore_ConcurrentCreate(t *testing.T) {
	db := tests.InitTransactionTestDB()
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
