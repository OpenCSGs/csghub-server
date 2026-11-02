package database_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"sync"
	"testing"
	"time"
)

// TestBillSummaryDBStore_ConcurrentCreateOrGetBillSummary
// Test multiple goroutines simultaneously attempting to create the same BillSummaryDB record.
// Expected results:
//  1. Only one goroutine successfully inserts the new record and returns it.
//  2. Other goroutines encounter ON CONFLICT DO NOTHING during insertion,
//     preventing duplicate records from being created. They subsequently retrieve
//     the already existing record.
//  3. Ultimately, all goroutines return the same record ID.
func TestBillSummaryDBStore_ConcurrentCreateOrGetBillSummary(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()

	store := database.NewBillSummaryDBStoreWithDB(db)

	// Base record information for testing
	summary := &database.BillSummaryDB{
		GatewayType: "test-gateway",
		Account:     "test-account",
		BillDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		S3Bucket:    "test-bucket",
		S3Key:       "test/key/path",
	}

	const goroutineCount = 10
	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	results := make(chan *database.BillSummaryDB, goroutineCount)
	errs := make(chan error, goroutineCount)

	// Launch multiple goroutines to concurrently call CreateOrGetBillSummary
	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wg.Done()
			res, err := store.CreateOrGetBillSummary(ctx, summary)
			results <- res
			errs <- err
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	var finalSummary *database.BillSummaryDB
	for r := range results {
		require.NotNil(t, r, "The returned result should not be nil")
		if finalSummary == nil {
			// Use the first result as the baseline
			finalSummary = r
			require.True(t, finalSummary.ID > 0, "ID should be valid and greater than 0 after insertion")
		} else {
			// All returned results should be the same record (same ID)
			require.Equal(t, finalSummary.ID, r.ID, "All returned records should have the same ID")
		}
	}

	for e := range errs {
		require.Nil(t, e, "All goroutine calls should return no errors")
	}

	require.NotNil(t, finalSummary, "A valid record should be returned ultimately")

	// Call again to verify that the same record is retrieved
	again, err := store.CreateOrGetBillSummary(ctx, summary)
	require.Nil(t, err)
	require.Equal(t, finalSummary.ID, again.ID, "Retrieving with the same parameters should return the same record ID")
}
