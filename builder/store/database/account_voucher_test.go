package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func createTestVoucher(targetUUID string, total float64) *database.AccountVoucher {
	return &database.AccountVoucher{
		TargetType: database.UserNamespace,
		TargetUUID: targetUUID,
		TargetName: "testuser",
		Total:      total,
		BeginDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		Status:     types.VoucherStatusPending,
		Rules:      []types.VoucherRules{{ClusterIDs: []string{"cluster-1"}}},
		Notes:      "test notes",
		IssueUUID:  "admin-uuid",
		IssueName:  "admin",
	}
}

func TestAccountVoucherStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-1", 100.0)

	result, err := store.Create(ctx, *voucher)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.VoucherNo)
	require.Equal(t, types.VoucherStatusPending, result.Status)
	require.Equal(t, "user-uuid-1", result.TargetUUID)
	require.Equal(t, 100.0, result.Total)
}

func TestAccountVoucherStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-2", 200.0)

	created, err := store.Create(ctx, *voucher)
	require.Nil(t, err)

	result, err := store.GetByID(ctx, created.ID)
	require.Nil(t, err)
	require.Equal(t, created.ID, result.ID)
	require.Equal(t, created.VoucherNo, result.VoucherNo)
	require.Equal(t, "user-uuid-2", result.TargetUUID)
}

func TestAccountVoucherStore_GetByVoucherNo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-3", 300.0)

	created, err := store.Create(ctx, *voucher)
	require.Nil(t, err)

	result, err := store.GetByVoucherNo(ctx, created.VoucherNo)
	require.Nil(t, err)
	require.Equal(t, created.ID, result.ID)
	require.Equal(t, created.VoucherNo, result.VoucherNo)
}

func TestAccountVoucherStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-4", 100.0)

	created, err := store.Create(ctx, *voucher)
	require.Nil(t, err)

	created.Total = 500.0
	created.Notes = "updated notes"
	result, err := store.Update(ctx, *created)
	require.Nil(t, err)
	require.Equal(t, 500.0, result.Total)
	require.Equal(t, "updated notes", result.Notes)
}

func TestAccountVoucherStore_UpdateStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-5", 100.0)

	created, err := store.Create(ctx, *voucher)
	require.Nil(t, err)

	result, err := store.UpdateStatus(ctx, created.ID, types.VoucherStatusActive)
	require.Nil(t, err)
	require.Equal(t, types.VoucherStatusActive, result.Status)
}

func TestAccountVoucherStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)
	voucher := createTestVoucher("user-uuid-6", 100.0)

	created, err := store.Create(ctx, *voucher)
	require.Nil(t, err)

	err = store.Delete(ctx, created.ID)
	require.Nil(t, err)

	result, err := store.GetByID(ctx, created.ID)
	require.NotNil(t, err)
	require.Nil(t, result)
}

func TestAccountVoucherStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)

	v1 := createTestVoucher("user-uuid-7", 100.0)
	v2 := createTestVoucher("user-uuid-7", 200.0)
	_, err := store.Create(ctx, *v1)
	require.Nil(t, err)
	_, err = store.Create(ctx, *v2)
	require.Nil(t, err)

	filter := types.VoucherFilter{
		TargetUUID: "user-uuid-7",
		Per:        10,
		Page:       1,
	}

	vouchers, total, err := store.List(ctx, filter)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, vouchers, 2)
}

func TestAccountVoucherStore_ListWithStatusFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)

	v1 := createTestVoucher("user-uuid-8", 100.0)
	v2 := createTestVoucher("user-uuid-8", 200.0)
	created1, err := store.Create(ctx, *v1)
	require.Nil(t, err)
	_, err = store.Create(ctx, *v2)
	require.Nil(t, err)

	_, err = store.UpdateStatus(ctx, created1.ID, types.VoucherStatusActive)
	require.Nil(t, err)

	filter := types.VoucherFilter{
		TargetUUID: "user-uuid-8",
		Status:     types.VoucherStatusActive,
		Per:        10,
		Page:       1,
	}

	vouchers, total, err := store.List(ctx, filter)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, vouchers, 1)
	require.Equal(t, types.VoucherStatusActive, vouchers[0].Status)
}

func TestAccountVoucherStore_GetDashboard(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)

	v1 := createTestVoucher("user-uuid-9", 100.0)
	v2 := createTestVoucher("user-uuid-9", 300.0)
	v3 := createTestVoucher("user-uuid-9", 200.0)
	created1, err := store.Create(ctx, *v1)
	require.Nil(t, err)
	created2, err := store.Create(ctx, *v2)
	require.Nil(t, err)
	_, err = store.Create(ctx, *v3)
	require.Nil(t, err)

	_, err = store.UpdateStatus(ctx, created1.ID, types.VoucherStatusActive)
	require.Nil(t, err)
	_, err = store.UpdateStatus(ctx, created2.ID, types.VoucherStatusActive)
	require.Nil(t, err)

	req := types.VoucherDashboardReq{
		TargetUUID: "user-uuid-9",
	}

	results, err := store.GetDashboard(ctx, req)
	require.Nil(t, err)
	require.Len(t, results, 2)

	statusMap := make(map[types.VoucherStatus]database.VoucherDashboardResult)
	for _, r := range results {
		statusMap[r.Status] = r
	}

	activeResult := statusMap[types.VoucherStatusActive]
	require.Equal(t, 400.0, activeResult.Total)
	require.Equal(t, 2, activeResult.Count)

	pendingResult := statusMap[types.VoucherStatusPending]
	require.Equal(t, 200.0, pendingResult.Total)
	require.Equal(t, 1, pendingResult.Count)
}

func TestAccountVoucherStore_GenVoucherNo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountVoucherStoreWithDB(db)

	v1 := createTestVoucher("user-uuid-10", 100.0)
	created1, err := store.Create(ctx, *v1)
	require.Nil(t, err)
	require.Regexp(t, `VC-\d{4}-\d{6}`, created1.VoucherNo)

	v2 := createTestVoucher("user-uuid-10", 200.0)
	created2, err := store.Create(ctx, *v2)
	require.Nil(t, err)
	require.Regexp(t, `VC-\d{4}-\d{6}`, created2.VoucherNo)
	require.NotEqual(t, created1.VoucherNo, created2.VoucherNo)
}
