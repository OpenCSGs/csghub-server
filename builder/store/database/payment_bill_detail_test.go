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

// TestBillDetailDBStore_CreateBillDetails_ConcurrentInsertion
// This test verifies that when multiple goroutines attempt to insert identical or overlapping sets of
// BillDetailDB records concurrently, the store avoids inserting duplicates due to the ON CONFLICT clause.
//
// Expected outcomes:
// 1. Only the unique records are ultimately inserted into the database.
// 2. Records that appear in multiple insertion requests are only inserted once.
// 3. No errors occur during concurrent insert operations.
func TestBillDetailDBStore_CreateBillDetails_ConcurrentInsertion(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewBillDetailDBStoreWithDB(db)

	// Prepare a base BillSummaryID to associate with details.
	// In a real scenario, you might ensure a corresponding BillSummary record exists.
	billSummaryID := int64(12345)

	// Create a set of details, with some duplicates.
	// We'll simulate duplicates by including the same PayOrderIDs in multiple sets.
	detailsSet1 := []*database.BillDetailDB{
		{
			BillSummaryID:   billSummaryID,
			PayOrderID:      "order-100",
			MerchantOrderID: "merchant-100",
			BusinessType:    "typeA",
			ProductName:     "ProductA",
			CreateTime:      time.Now(),
			CompleteTime:    time.Now(),
			PayUser:         "user1",
			OrderAmount:     100.0,
			MerchantReceive: 90.0,
			ServiceFee:      10.0,
			Currency:        "CNY",
		},
		{
			BillSummaryID:   billSummaryID,
			PayOrderID:      "order-101",
			MerchantOrderID: "merchant-101",
			BusinessType:    "typeB",
			ProductName:     "ProductB",
			CreateTime:      time.Now(),
			CompleteTime:    time.Now(),
			PayUser:         "user2",
			OrderAmount:     200.0,
			MerchantReceive: 180.0,
			ServiceFee:      20.0,
			Currency:        "CNY",
		},
	}

	detailsSet2 := []*database.BillDetailDB{
		// "order-101" is duplicated here
		{
			BillSummaryID:   billSummaryID,
			PayOrderID:      "order-101",
			MerchantOrderID: "merchant-101",
			BusinessType:    "typeB",
			ProductName:     "ProductB",
			CreateTime:      time.Now(),
			CompleteTime:    time.Now(),
			PayUser:         "user2",
			OrderAmount:     200.0,
			MerchantReceive: 180.0,
			ServiceFee:      20.0,
			Currency:        "CNY",
		},
		{
			BillSummaryID:   billSummaryID,
			PayOrderID:      "order-102",
			MerchantOrderID: "merchant-102",
			BusinessType:    "typeC",
			ProductName:     "ProductC",
			CreateTime:      time.Now(),
			CompleteTime:    time.Now(),
			PayUser:         "user3",
			OrderAmount:     300.0,
			MerchantReceive: 270.0,
			ServiceFee:      30.0,
			Currency:        "CNY",
		},
	}

	// We'll run multiple goroutines that call CreateBillDetails with overlapping sets.
	const goroutineCount = 5
	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	errs := make(chan error, goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(i int) {
			defer wg.Done()
			// Alternate between two sets to create overlap
			var err error
			if i%2 == 0 {
				err = store.CreateBillDetails(ctx, detailsSet1)
			} else {
				err = store.CreateBillDetails(ctx, detailsSet2)
			}
			errs <- err
		}(i)
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		require.NoError(t, e, "No errors should occur during concurrent insert operations")
	}

	// Verify that the database contains only the unique records: order-100, order-101, and order-102.
	var finalRecords []database.BillDetailDB
	err := db.Operator.Core.NewSelect().
		Model(&finalRecords).
		Where("bill_summary_id = ?", billSummaryID).
		Order("pay_order_id ASC").
		Scan(ctx)
	require.NoError(t, err, "Select query should succeed")

	require.Len(t, finalRecords, 3, "There should be exactly 3 unique records inserted")

	payOrderIDs := []string{finalRecords[0].PayOrderID, finalRecords[1].PayOrderID, finalRecords[2].PayOrderID}
	require.Contains(t, payOrderIDs, "order-100", "The first record set's unique order should exist")
	require.Contains(t, payOrderIDs, "order-101", "The common order present in both sets should only appear once")
	require.Contains(t, payOrderIDs, "order-102", "The second record set's unique order should exist")
}
