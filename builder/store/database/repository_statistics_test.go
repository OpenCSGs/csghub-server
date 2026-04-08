package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRepositoryStatisticsStore_Create(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create a test repository statistics
	stats := &database.RepositoryStatistics{
		RepositoryID: 1,
		TotalSize:    1024,
		NonLfsSize:   512,
		LfsSize:      512,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Test Create method
	err := store.Create(ctx, stats)
	assert.NoError(t, err)
	assert.Greater(t, stats.ID, int64(0))
}

func TestRepositoryStatisticsStore_FindByRepositoryID(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create a test repository statistics
	expectedStats := &database.RepositoryStatistics{
		RepositoryID: 2,
		TotalSize:    2048,
		NonLfsSize:   1024,
		LfsSize:      1024,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create the statistics
	err := store.Create(ctx, expectedStats)
	assert.NoError(t, err)

	// Test FindByRepositoryID method
	actualStats, err := store.FindByRepositoryID(ctx, expectedStats.RepositoryID)
	assert.NoError(t, err)
	assert.NotNil(t, actualStats)
	assert.Equal(t, expectedStats.RepositoryID, actualStats.RepositoryID)
	assert.Equal(t, expectedStats.TotalSize, actualStats.TotalSize)
	assert.Equal(t, expectedStats.NonLfsSize, actualStats.NonLfsSize)
	assert.Equal(t, expectedStats.LfsSize, actualStats.LfsSize)
}

func TestRepositoryStatisticsStore_Update(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create a test repository statistics
	stats := &database.RepositoryStatistics{
		RepositoryID: 3,
		TotalSize:    1024,
		NonLfsSize:   512,
		LfsSize:      512,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create the statistics
	err := store.Create(ctx, stats)
	assert.NoError(t, err)

	// Update the statistics
	updatedSize := int64(2048)
	stats.TotalSize = updatedSize
	stats.NonLfsSize = updatedSize / 2
	stats.LfsSize = updatedSize / 2
	stats.UpdatedAt = time.Now()

	// Test Update method
	err = store.Update(ctx, stats)
	assert.NoError(t, err)

	// Verify the update
	actualStats, err := store.FindByRepositoryID(ctx, stats.RepositoryID)
	assert.NoError(t, err)
	assert.Equal(t, updatedSize, actualStats.TotalSize)
}

func TestRepositoryStatisticsStore_Delete(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create a test repository statistics
	stats := &database.RepositoryStatistics{
		RepositoryID: 4,
		TotalSize:    1024,
		NonLfsSize:   512,
		LfsSize:      512,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create the statistics
	err := store.Create(ctx, stats)
	assert.NoError(t, err)

	// Test Delete method
	err = store.Delete(ctx, stats)
	assert.NoError(t, err)

	// Verify the deletion
	_, err = store.FindByRepositoryID(ctx, stats.RepositoryID)
	assert.Error(t, err)
}

func TestRepositoryStatisticsStore_BatchUpdate(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create multiple test repository statistics
	stats1 := &database.RepositoryStatistics{
		RepositoryID: 5,
		TotalSize:    1024,
		NonLfsSize:   512,
		LfsSize:      512,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	stats2 := &database.RepositoryStatistics{
		RepositoryID: 6,
		TotalSize:    2048,
		NonLfsSize:   1024,
		LfsSize:      1024,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create the statistics
	err := store.Create(ctx, stats1)
	assert.NoError(t, err)

	err = store.Create(ctx, stats2)
	assert.NoError(t, err)

	// Update the statistics
	updatedSize1 := int64(2048)
	stats1.TotalSize = updatedSize1
	stats1.NonLfsSize = updatedSize1 / 2
	stats1.LfsSize = updatedSize1 / 2
	stats1.UpdatedAt = time.Now()

	updatedSize2 := int64(4096)
	stats2.TotalSize = updatedSize2
	stats2.NonLfsSize = updatedSize2 / 2
	stats2.LfsSize = updatedSize2 / 2
	stats2.UpdatedAt = time.Now()

	// Test BatchUpdate method
	err = store.BatchUpdate(ctx, []*database.RepositoryStatistics{stats1, stats2})
	assert.NoError(t, err)

	// Verify the updates
	actualStats1, err := store.FindByRepositoryID(ctx, stats1.RepositoryID)
	assert.NoError(t, err)
	assert.Equal(t, updatedSize1, actualStats1.TotalSize)

	actualStats2, err := store.FindByRepositoryID(ctx, stats2.RepositoryID)
	assert.NoError(t, err)
	assert.Equal(t, updatedSize2, actualStats2.TotalSize)
}

func TestRepositoryStatisticsStore_FindByRepositoryIDAndBranch(t *testing.T) {
	ctx := context.Background()
	// Create a test database
	db := tests.InitTestDB()
	defer db.Close()

	// Create a repository statistics store with test database
	store := database.NewRepositoryStatisticsStoreWithDB(db)

	// Create a test repository statistics with branch
	expectedStats := &database.RepositoryStatistics{
		RepositoryID: 7,
		Branch:       "main",
		TotalSize:    1024,
		NonLfsSize:   512,
		LfsSize:      512,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create the statistics
	err := store.Create(ctx, expectedStats)
	assert.NoError(t, err)

	// Test FindByRepositoryIDAndBranch method
	actualStats, err := store.FindByRepositoryIDAndBranch(ctx, expectedStats.RepositoryID, expectedStats.Branch)
	assert.NoError(t, err)
	assert.NotNil(t, actualStats)
	assert.Equal(t, expectedStats.RepositoryID, actualStats.RepositoryID)
	assert.Equal(t, expectedStats.Branch, actualStats.Branch)
	assert.Equal(t, expectedStats.TotalSize, actualStats.TotalSize)
	assert.Equal(t, expectedStats.NonLfsSize, actualStats.NonLfsSize)
	assert.Equal(t, expectedStats.LfsSize, actualStats.LfsSize)

	// Test with non-existent branch
	_, err = store.FindByRepositoryIDAndBranch(ctx, expectedStats.RepositoryID, "non-existent-branch")
	assert.Error(t, err)
}
