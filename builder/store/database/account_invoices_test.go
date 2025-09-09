package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
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

func TestCreateInvoice(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	invoice := createTestInvoice()

	err := store.CreateInvoice(ctx, invoice)
	require.Nil(t, err)
}

func TestUpdateInvoice(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountInvoiceStoreWithDB(db)
	invoice := createTestInvoice()

	// First create the invoice
	err := store.CreateInvoice(ctx, invoice)
	require.Nil(t, err)

	// Update the invoice information
	invoice.InvoiceAmount = 200.0
	err = store.UpdateInvoice(ctx, invoice)
	require.Nil(t, err)
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

	for _, invoice := range testInvoices {
		err := store.CreateInvoice(ctx, invoice)
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
