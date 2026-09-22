package database_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/utils/payment/consts"
)

// createTestInvoice creates a test invoice instance.
func createTestInvoice() *database.AccountInvoice {
	return &database.AccountInvoice{
		UserUUID:       "test-user",
		TitleType:      database.TitleTypeEnterpriseOrdinary,
		InvoiceType:    database.InvoiceTypeOrdinary,
		BillCycle:      "2024-04",
		InvoiceTitle:   "Test Invoice",
		ApplyTime:      time.Now(),
		InvoiceAmount:  100.0,
		Status:         database.InvoiceStatusProcessing,
		Reason:         "",
		InvoiceDate:    time.Now(),
		InvoiceURL:     "https://example.com/invoice.pdf",
		TaxpayerID:     "1234567890",
		BankName:       "Test Bank",
		BankAccount:    "1234567890123456",
		RegisteredAddr: "Test Address",
		ContactPhone:   "1234567890",
		Email:          "test@example.com",
	}
}

// createTestRecharge creates a paid test recharge instance.
func createTestRecharge(uuid string) *database.AccountRecharge {
	return &database.AccountRecharge{
		RechargeUUID:  uuid,
		OrderNo:       "order-" + uuid,
		UserUUID:      "test-user",
		FromUserUUID:  "test-user",
		Amount:        10000,
		Currency:      "CNY",
		Channel:       consts.ChannelAlipay,
		PaymentUUID:   "payment-" + uuid,
		Succeeded:     true,
		TimeSucceeded: time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC),
	}
}

// persistTestRecharges inserts the given recharges into account_recharges so
// that invoice creation can lock and validate them.
func persistTestRecharges(t *testing.T, db *database.DB, recharges ...*database.AccountRecharge) {
	t.Helper()
	rechargeStore := database.NewAccountRechargeStoreWithDB(db)
	for _, recharge := range recharges {
		require.Nil(t, rechargeStore.CreateRecharge(context.TODO(), recharge))
	}
}

// createTestInvoiceTitle creates a test invoice title instance.
func createTestInvoiceTitle() *database.AccountInvoiceTitle {
	return &database.AccountInvoiceTitle{
		UserUUID:     "test-user",
		Title:        "Test Title",
		TitleType:    database.TitleTypeEnterpriseOrdinary,
		TaxID:        "1234567890",
		Address:      "Test Address",
		BankName:     "Test Bank",
		BankAccount:  "1234567890123456",
		ContactPhone: "1234567890",
		Email:        "test@example.com",
		IsDefault:    true,
	}
}

func TestCreateInvoiceWithRecharges(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	invoice := createTestInvoice()
	invoice.BillCycle = "" // recharge-based invoice
	recharges := []*database.AccountRecharge{createTestRecharge("r-1"), createTestRecharge("r-2")}
	persistTestRecharges(t, db, recharges...)

	err := store.CreateInvoiceWithRecharges(ctx, invoice, recharges)
	require.Nil(t, err)
	require.NotZero(t, invoice.ID)

	links, err := store.ListInvoiceRecharges(ctx, int64(invoice.ID))
	require.Nil(t, err)
	require.Len(t, links, 2)
	require.Equal(t, "order-r-1", links[0].OrderNo)
	require.Equal(t, "order-r-2", links[1].OrderNo)
	require.Equal(t, int64(10000), links[0].AmountCents)
	require.Equal(t, "CNY", links[0].Currency)
	require.Equal(t, "test-user", links[0].UserUUID)
	require.Equal(t, string(consts.ChannelAlipay), links[0].PaymentChannel)
	require.Equal(t, recharges[0].TimeSucceeded, links[0].RechargeTime.Time)
}

func TestCreateInvoiceWithRechargesRequiresRecharges(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	err := store.CreateInvoiceWithRecharges(ctx, createTestInvoice(), nil)
	require.NotNil(t, err)
}

func TestCreateInvoiceWithRechargesRetryAfterFailed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	recharge := createTestRecharge("r-retry")
	persistTestRecharges(t, db, recharge)

	first := createTestInvoice()
	first.BillCycle = ""
	require.Nil(t, store.CreateInvoiceWithRecharges(ctx, first, []*database.AccountRecharge{recharge}))

	// admin rejects the first invoice, which frees the order for a new
	// application; the same order may be linked again
	first.Status = database.InvoiceStatusFailed
	first.Reason = "wrong title"
	require.Nil(t, store.UpdateInvoice(ctx, first))

	second := createTestInvoice()
	second.BillCycle = ""
	second.InvoiceTitle = "Fixed Title"
	require.Nil(t, store.CreateInvoiceWithRecharges(ctx, second, []*database.AccountRecharge{recharge}))

	links, err := store.ListInvoiceRecharges(ctx, int64(second.ID))
	require.Nil(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "order-r-retry", links[0].OrderNo)
}

func TestCreateInvoiceWithRechargesRejectsAlreadyInvoiced(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	recharge := createTestRecharge("r-dup")
	persistTestRecharges(t, db, recharge)

	first := createTestInvoice()
	first.BillCycle = ""
	require.Nil(t, store.CreateInvoiceWithRecharges(ctx, first, []*database.AccountRecharge{recharge}))

	// the order is already attached to the processing invoice above
	second := createTestInvoice()
	second.BillCycle = ""
	err := store.CreateInvoiceWithRecharges(ctx, second, []*database.AccountRecharge{recharge})
	require.ErrorIs(t, err, database.ErrRechargeNotInvoicable)
}

func TestCreateInvoiceWithRechargesConcurrent(t *testing.T) {
	// InitTestDB brings up (and migrates) the shared test container; the
	// txdb-wrapped DB it returns multiplexes every query through a single
	// backend session, so cross-session locks are invisible to it. The
	// concurrency scenario below therefore opens a real connection instead.
	tests.InitTestDB()

	ctx := context.TODO()
	realDB, err := database.NewDB(context.TODO(), database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     tests.TestDBDSN() + "sslmode=disable",
	})
	require.Nil(t, err)
	defer realDB.Close()

	store := database.NewAccountInvoiceStoreWithDB(realDB)
	recharge := createTestRecharge(fmt.Sprintf("r-concurrent-%d", time.Now().UnixNano()))
	persistTestRecharges(t, realDB, recharge)
	// rows committed through the real connection are not rolled back by
	// txdb, so clean them up to keep other tests' expectations intact
	defer func() {
		_, err := realDB.Core.NewRaw("DELETE FROM account_invoice_recharges WHERE recharge_uuid = ?", recharge.RechargeUUID).Exec(ctx)
		if err != nil {
			t.Errorf("clean up invoice-recharge links: %v", err)
		}
		_, err = realDB.Core.NewRaw("DELETE FROM account_recharges WHERE recharge_uuid = ?", recharge.RechargeUUID).Exec(ctx)
		if err != nil {
			t.Errorf("clean up recharges: %v", err)
		}
	}()

	// two concurrent applications for the same order must produce exactly
	// one invoice; the per-user advisory lock serializes them so the loser
	// re-checks invoicability on a fresh snapshot and fails cleanly
	const attempts = 2
	results := make(chan error, attempts)
	created := make(chan *database.AccountInvoice, attempts)
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		go func() {
			<-start
			invoice := createTestInvoice()
			invoice.BillCycle = "" // recharge-based invoice
			created <- invoice
			results <- store.CreateInvoiceWithRecharges(ctx, invoice, []*database.AccountRecharge{recharge})
		}()
	}
	close(start)
	var succeeded, rejected int
	for i := 0; i < attempts; i++ {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, database.ErrRechargeNotInvoicable):
			rejected++
		default:
			t.Fatalf("unexpected concurrent creation error: %v", err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, attempts-1, rejected)
	// release the committed invoice rows before the deferred recharge cleanup
	close(created)
	for invoice := range created {
		if invoice.ID != 0 {
			_, err := realDB.Core.NewRaw("DELETE FROM account_invoice_recharges WHERE invoice_id = ?", invoice.ID).Exec(ctx)
			if err != nil {
				t.Errorf("clean up links of invoice %d: %v", invoice.ID, err)
			}
			_, err = realDB.Core.NewRaw("DELETE FROM account_invoices WHERE id = ?", invoice.ID).Exec(ctx)
			if err != nil {
				t.Errorf("clean up invoice %d: %v", invoice.ID, err)
			}
		}
	}
}

func TestUpdateInvoice(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	recharge := createTestRecharge("r-update")
	persistTestRecharges(t, db, recharge)
	invoice := createTestInvoice()
	err := store.CreateInvoiceWithRecharges(ctx, invoice, []*database.AccountRecharge{recharge})
	require.Nil(t, err)

	invoice.InvoiceAmount = 200.0
	err = store.UpdateInvoice(ctx, invoice)
	require.Nil(t, err)

	updated, err := store.GetInvoice(ctx, int64(invoice.ID))
	require.Nil(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 200.0, updated.InvoiceAmount)
}

func TestUpdateInvoiceNotFailed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	recharge := createTestRecharge("r-update-notfailed")
	persistTestRecharges(t, db, recharge)
	invoice := createTestInvoice()
	err := store.CreateInvoiceWithRecharges(ctx, invoice, []*database.AccountRecharge{recharge})
	require.Nil(t, err)

	// while not failed the update goes through, including the transition to
	// the terminal failed state itself
	invoice.InvoiceAmount = 200.0
	invoice.Status = database.InvoiceStatusFailed
	invoice.Reason = "rejected by tax bureau"
	updated, err := store.UpdateInvoiceNotFailed(ctx, invoice)
	require.Nil(t, err)
	require.True(t, updated)

	// a later update trying to revive the failed invoice updates no rows
	invoice.Status = database.InvoiceStatusIssued
	invoice.InvoiceURL = "https://example.com/inv.pdf"
	updated, err = store.UpdateInvoiceNotFailed(ctx, invoice)
	require.Nil(t, err)
	require.False(t, updated)

	stored, err := store.GetInvoice(ctx, int64(invoice.ID))
	require.Nil(t, err)
	require.NotNil(t, stored)
	require.Equal(t, database.InvoiceStatusFailed, stored.Status)
	require.Equal(t, "rejected by tax bureau", stored.Reason)
}

func TestListInvoices(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)

	// Insert test invoice data
	testInvoices := []*database.AccountInvoice{
		createTestInvoice(),
		createTestInvoice(),
	}
	testInvoices[1].InvoiceTitle = "Test Invoice 2"
	testInvoices[1].BillCycle = "2024-04"

	persistTestRecharges(t, db,
		createTestRecharge("r-list-0"),
		createTestRecharge("r-list-1"),
	)
	for i, invoice := range testInvoices {
		err := store.CreateInvoiceWithRecharges(ctx, invoice, []*database.AccountRecharge{createTestRecharge(fmt.Sprintf("r-list-%d", i))})
		require.Nil(t, err)
	}

	params := database.InvoiceListParams{
		UserUUID: "test-user",
		Page:     1,
		PageSize: 10,
	}

	invoices, count, err := store.ListInvoices(ctx, params)
	require.Nil(t, err)
	require.GreaterOrEqual(t, count, 1) // At least one record
	require.NotNil(t, invoices)
	require.GreaterOrEqual(t, len(invoices), 1) // Return at least one invoice record
}

func TestGetInvoicableRecharges(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	invoiceStore := database.NewAccountInvoiceStoreWithDB(db)
	rechargeStore := database.NewAccountRechargeStoreWithDB(db)

	recharges := map[string]*database.AccountRecharge{
		"paid":     createTestRecharge("r-paid"),     // succeeded, not invoiced
		"unpaid":   createTestRecharge("r-unpaid"),   // succeeded=false
		"closed":   createTestRecharge("r-closed"),   // closed=true
		"invoiced": createTestRecharge("r-invoiced"), // attached to an issued invoice
		"failed":   createTestRecharge("r-failed"),   // attached to a failed invoice, still invoicable
		"other":    createTestRecharge("r-other"),    // another user
	}
	recharges["unpaid"].Succeeded = false
	recharges["closed"].Closed = true
	recharges["other"].UserUUID = "other-user"
	recharges["paid"].TimeSucceeded = time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)
	recharges["failed"].TimeSucceeded = time.Date(2024, 4, 20, 12, 0, 0, 0, time.UTC)
	for _, recharge := range recharges {
		require.Nil(t, rechargeStore.CreateRecharge(ctx, recharge))
	}

	// attach recharges to invoices so the exclusion subquery kicks in
	issued := createTestInvoice()
	issued.Status = database.InvoiceStatusIssued
	issued.BillCycle = ""
	require.Nil(t, invoiceStore.CreateInvoiceWithRecharges(ctx, issued, []*database.AccountRecharge{recharges["invoiced"]}))
	failed := createTestInvoice()
	failed.Status = database.InvoiceStatusFailed
	failed.BillCycle = ""
	require.Nil(t, invoiceStore.CreateInvoiceWithRecharges(ctx, failed, []*database.AccountRecharge{recharges["failed"]}))

	t.Run("Only paid, unclosed and not invoiced recharges of the user", func(t *testing.T) {
		rows, count, err := invoiceStore.GetInvoicableRecharges(ctx, database.InvoicableRechargeFilter{UserUUID: "test-user"})
		require.Nil(t, err)
		require.Equal(t, 2, count)
		require.Len(t, rows, 2)
		orderNos := []string{rows[0].OrderNo, rows[1].OrderNo}
		require.ElementsMatch(t, []string{"order-r-paid", "order-r-failed"}, orderNos)
	})

	t.Run("Filter by order numbers", func(t *testing.T) {
		rows, count, err := invoiceStore.GetInvoicableRecharges(ctx, database.InvoicableRechargeFilter{
			UserUUID: "test-user",
			OrderNos: []string{"order-r-paid"},
		})
		require.Nil(t, err)
		require.Equal(t, 1, count)
		require.Len(t, rows, 1)
		require.Equal(t, "order-r-paid", rows[0].OrderNo)
	})

	t.Run("Filter by month range", func(t *testing.T) {
		rows, count, err := invoiceStore.GetInvoicableRecharges(ctx, database.InvoicableRechargeFilter{
			UserUUID:   "test-user",
			StartMonth: "2024-04",
			EndMonth:   "2024-05",
		})
		require.Nil(t, err)
		require.Equal(t, 1, count)
		require.Len(t, rows, 1)
		require.Equal(t, "order-r-failed", rows[0].OrderNo)
	})

	t.Run("Paginate", func(t *testing.T) {
		rows, count, err := invoiceStore.GetInvoicableRecharges(ctx, database.InvoicableRechargeFilter{
			UserUUID: "test-user",
			Page:     1,
			PageSize: 1,
		})
		require.Nil(t, err)
		require.Equal(t, 2, count)
		require.Len(t, rows, 1)
	})
}

func TestListInvoiceRechargesByInvoiceIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	persistTestRecharges(t, db,
		createTestRecharge("r-batch-1"),
		createTestRecharge("r-batch-2"),
		createTestRecharge("r-batch-3"),
	)

	invoice1 := createTestInvoice()
	invoice1.BillCycle = ""
	require.Nil(t, store.CreateInvoiceWithRecharges(ctx, invoice1, []*database.AccountRecharge{
		createTestRecharge("r-batch-1"),
		createTestRecharge("r-batch-2"),
	}))
	invoice2 := createTestInvoice()
	invoice2.BillCycle = ""
	require.Nil(t, store.CreateInvoiceWithRecharges(ctx, invoice2, []*database.AccountRecharge{
		createTestRecharge("r-batch-3"),
	}))

	t.Run("List across invoices ordered by link id", func(t *testing.T) {
		links, err := store.ListInvoiceRechargesByInvoiceIDs(ctx, []int64{int64(invoice1.ID), int64(invoice2.ID)})
		require.Nil(t, err)
		require.Len(t, links, 3)
		require.Equal(t, []string{"order-r-batch-1", "order-r-batch-2", "order-r-batch-3"},
			[]string{links[0].OrderNo, links[1].OrderNo, links[2].OrderNo})
	})

	t.Run("Empty invoice ids returns no links", func(t *testing.T) {
		links, err := store.ListInvoiceRechargesByInvoiceIDs(ctx, nil)
		require.Nil(t, err)
		require.Empty(t, links)
	})
}

func TestCreateInvoiceTitle(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	title := createTestInvoiceTitle()

	err := store.CreateInvoiceTitle(ctx, title)
	require.Nil(t, err)
}

func TestUpdateInvoiceTitle(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	title := createTestInvoiceTitle()

	// First create the invoice title
	err := store.CreateInvoiceTitle(ctx, title)
	require.Nil(t, err)

	// Update the invoice title information
	title.Title = "Updated Test Title"
	err = store.UpdateInvoiceTitle(ctx, title)
	require.Nil(t, err)
}

func TestListInvoiceTitles(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)

	// Insert test invoice title data
	testTitles := []*database.AccountInvoiceTitle{
		{
			UserUUID:     "test-user",
			Title:        "Test Title 1",
			TitleType:    database.TitleTypeEnterpriseOrdinary,
			TaxID:        "1234567890",
			Address:      "Test Address 1",
			BankName:     "Test Bank",
			BankAccount:  "1234567890123456",
			ContactPhone: "1234567890",
			Email:        "test@example.com",
			IsDefault:    true,
		},
		{
			UserUUID:     "test-user",
			Title:        "Test Title 2",
			TitleType:    database.TitleTypeEnterpriseOrdinary,
			TaxID:        "0987654321",
			Address:      "Test Address 2",
			BankName:     "Test Bank",
			BankAccount:  "6543210987654321",
			ContactPhone: "0987654321",
			Email:        "test2@example.com",
			IsDefault:    false,
		},
	}

	for _, title := range testTitles {
		err := store.CreateInvoiceTitle(ctx, title)
		require.Nil(t, err)
	}

	params := database.InvoiceListParams{
		UserUUID: "test-user",
		Page:     1,
		PageSize: 10,
	}

	titles, count, err := store.ListInvoiceTitles(ctx, params)
	require.Nil(t, err)
	require.GreaterOrEqual(t, count, 1) // At least one record
	require.NotNil(t, titles)
	require.GreaterOrEqual(t, len(titles), 1) // Return at least one invoice title record
}
