package database_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

const tokenPlanDay = int64(24 * 60 * 60)

func TestAccountTokenPlanStore_GetActiveInstanceByNsUUID_FiltersEndedPeriod(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	now := time.Now().Unix()
	instances := []database.AccountTokenPlanInstance{
		{ID: 1, NsUUID: "ns-1", EndAt: now - 100, Status: types.SubscriptionStatusActive},
		{ID: 2, NsUUID: "ns-1", EndAt: now + 100, Status: types.SubscriptionStatusActive},
		{ID: 3, NsUUID: "ns-2", EndAt: now + 100, Status: types.SubscriptionStatusActive},
		{ID: 4, NsUUID: "ns-1", EndAt: now + 100, Status: types.SubscriptionStatusExpired},
		{ID: 5, NsUUID: "ns-1", PlanID: 3, EndAt: 0, Status: types.SubscriptionStatusActive},
	}
	for i := range instances {
		_, err := db.Core.NewInsert().Model(&instances[i]).Exec(ctx, &instances[i])
		require.NoError(t, err)
	}

	// The instance without end time is renewed by the reconciliation, so it needs
	// a plan version to bind and an account to charge. The free plan keeps the
	// renewal away from the account balance.
	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  3,
		Version: 1,
	}
	_, err := db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.AccountUser{UserUUID: "ns-1"}).Exec(ctx)
	require.NoError(t, err)

	// The active instance with a future end time is not reconciled, so it needs
	// its own current period to satisfy the latest-period lookup.
	_, err = db.Core.NewInsert().Model(&database.AccountTokenPlanInstancePeriod{
		InstanceID:    2,
		PlanVersionID: version.ID,
		PeriodNo:      1,
		StartAt:       now - 100,
		EndAt:         now + 100,
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCreate,
	}).Exec(ctx)
	require.NoError(t, err)

	store := database.NewAccountTokenPlanStoreWithDB(db)
	result, err := store.GetActiveInstanceByNsUUID(ctx, types.AcctTokenPlanNSGetReq{
		NsUUID:      "ns-1",
		RequestTime: time.Now(),
		EventUUID:   uuid.New(),
	})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, int64(5), result[0].ID)
	require.Equal(t, int64(2), result[1].ID)

	var endedInstance database.AccountTokenPlanInstance
	err = db.Core.NewSelect().Model(&endedInstance).Where("id = ?", 1).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, types.SubscriptionStatusCanceled, endedInstance.Status)
}

func TestAccountTokenPlanStore_GetActiveInstanceByNsUUID_LoadsLatestPeriodUsages(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	now := time.Now().Unix()
	nsUUID := "ns-usage-" + uuid.NewString()
	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             21,
		FirstPlanVersionID: 1,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})

	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  21,
		Version: 1,
	}
	_, err := db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	period := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    instance.ID,
		PlanVersionID: version.ID,
		PeriodNo:      1,
		StartAt:       now - tokenPlanDay,
		EndAt:         now + 10*tokenPlanDay,
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCreate,
	})

	modelQuota := database.AccountTokenPlanPeriodModelQuota{
		PeriodID:          period.ID,
		ModelID:           "model-a",
		TaskType:          "text-generation",
		QuotaTokens:       1000,
		UsedTokens:        100,
		QuotaRequestCount: 10,
		UsedRequestCount:  1,
	}
	_, err = db.Core.NewInsert().Model(&modelQuota).Exec(ctx)
	require.NoError(t, err)

	keyQuota := database.AccountTokenPlanPeriodKeyQuota{
		PeriodID:          period.ID,
		TokenID:           7,
		ModelID:           "model-a",
		TaskType:          "text-generation",
		QuotaTokens:       500,
		UsedTokens:        50,
		QuotaRequestCount: 5,
		UsedRequestCount:  1,
	}
	_, err = db.Core.NewInsert().Model(&keyQuota).Exec(ctx)
	require.NoError(t, err)

	store := database.NewAccountTokenPlanStoreWithDB(db)
	result, err := store.GetActiveInstanceByNsUUID(ctx, types.AcctTokenPlanNSGetReq{
		NsUUID:      nsUUID,
		RequestTime: time.Now(),
		EventUUID:   uuid.New(),
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.NotNil(t, result[0].LatestPeriod)
	require.Len(t, result[0].LatestPeriod.ModelsUsage, 1)
	require.Equal(t, int64(100), result[0].LatestPeriod.ModelsUsage[0].UsedTokens)
	require.Len(t, result[0].LatestPeriod.APIKeysUsage, 1)
	require.Equal(t, int64(50), result[0].LatestPeriod.APIKeysUsage[0].UsedTokens)
}

func TestAccountTokenPlanStore_RenewsPeriodInGlobalTimeZone(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	shanghai := time.FixedZone("CST", 8*60*60)
	prevTz := config.GetGlobalTimeZone()
	config.SetGlobalTimeZone(shanghai)
	t.Cleanup(func() { config.SetGlobalTimeZone(prevTz) })

	nsUUID := "ns-tz-" + uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: nsUUID}).Exec(ctx)
	require.NoError(t, err)

	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  31,
		Version: 1,
	}
	_, err = db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             31,
		FirstPlanVersionID: version.ID,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})

	prevStart := time.Date(2026, 3, 1, 0, 30, 0, 0, shanghai)
	prevEnd := time.Date(2026, 4, 1, 0, 30, 0, 0, shanghai)
	insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    instance.ID,
		PlanVersionID: version.ID,
		PeriodNo:      1,
		StartAt:       prevStart.Unix(),
		EndAt:         prevEnd.Unix(),
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCreate,
	})

	store := database.NewAccountTokenPlanStoreWithDB(db)
	_, err = store.GetActiveInstanceByNsUUID(ctx, types.AcctTokenPlanNSGetReq{
		NsUUID:      nsUUID,
		RequestTime: prevEnd.Add(time.Minute),
		EventUUID:   uuid.New(),
	})
	require.NoError(t, err)

	var renewed database.AccountTokenPlanInstancePeriod
	err = db.Core.NewSelect().Model(&renewed).
		Where("instance_id = ?", instance.ID).
		Order("period_no DESC").Limit(1).
		Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), renewed.PeriodNo)
	require.Equal(t, prevEnd.Unix(), renewed.StartAt)

	wantEnd := time.Date(2026, 5, 1, 0, 30, 0, 0, shanghai)
	require.Equal(t, wantEnd.Unix(), renewed.EndAt)
}

func TestAccountTokenPlanStore_AutoScanNonBalanceFailureDoesNotAdvancePeriod(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	nsUUID := "ns-deduct-failed-" + uuid.NewString()
	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PriceAmount:   100,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  410,
		Version: 1,
	}
	_, err := db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})

	prevEnd := time.Now().Add(-time.Hour)
	insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    instance.ID,
		PlanVersionID: version.ID,
		PeriodNo:      1,
		StartAt:       prevEnd.Add(-30 * 24 * time.Hour).Unix(),
		EndAt:         prevEnd.Unix(),
		Amount:        100,
		Status:        types.BillingStatusUnpaid,
		Reason:        types.BillingReasonDeductFailed,
	})

	store := database.NewAccountTokenPlanStoreWithDB(db)
	requestTime := prevEnd.Add(time.Minute)
	result, err := store.GetActiveInstanceByNsUUID(ctx, types.AcctTokenPlanNSGetReq{
		NsUUID:      nsUUID,
		RequestTime: requestTime,
		EventUUID:   uuid.New(),
	})
	require.NoError(t, err)
	require.Empty(t, result)

	var storedInstance database.AccountTokenPlanInstance
	err = db.Core.NewSelect().Model(&storedInstance).Where("id = ?", instance.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, types.SubscriptionStatusExpired, storedInstance.Status)
	require.Equal(t, requestTime.Unix(), storedInstance.ExpiredAt)

	var periods []database.AccountTokenPlanInstancePeriod
	err = db.Core.NewSelect().Model(&periods).Where("instance_id = ?", instance.ID).Order("period_no ASC").Scan(ctx)
	require.NoError(t, err)
	require.Len(t, periods, 1)
	require.Equal(t, int64(1), periods[0].PeriodNo)
	require.Equal(t, types.BillingReasonDeductFailed, periods[0].Reason)
}

func TestAccountTokenPlanStore_Summary(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	newPlan := func(code, subjectType string, status types.TokenPlanStatus) database.AccountTokenPlan {
		return database.AccountTokenPlan{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:    code,
				PlanName:    code,
				SubjectType: database.NamespaceType(subjectType),
				Status:      status,
				CreatedBy:   "tester",
				Quotas:      []types.AccountTokenPlanModelQuota{},
				NsUUIDs:     []string{},
			},
		}
	}

	plans := []database.AccountTokenPlan{
		newPlan("summary-user-draft-1", string(database.UserNamespace), types.TokenPlanStatusDraft),
		newPlan("summary-user-draft-2", string(database.UserNamespace), types.TokenPlanStatusDraft),
		newPlan("summary-org-published", string(database.OrgNamespace), types.TokenPlanStatusPublished),
		newPlan("summary-blank-disabled", "", types.TokenPlanStatusDisabled),
	}
	deleted := newPlan("summary-deleted", string(database.UserNamespace), types.TokenPlanStatusDraft)
	deleted.DeletedAt = time.Now()
	plans = append(plans, deleted)

	for i := range plans {
		_, err := db.Core.NewInsert().Model(&plans[i]).Exec(ctx, &plans[i])
		require.NoError(t, err)
	}

	store := database.NewAccountTokenPlanStoreWithDB(db)
	res, err := store.Summary(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.AcctTokenPlanSummaryItem{
		{Status: types.TokenPlanStatusDraft, SubjectType: string(database.UserNamespace), Count: 2},
		{Status: types.TokenPlanStatusPublished, SubjectType: string(database.OrgNamespace), Count: 1},
		{Status: types.TokenPlanStatusDisabled, SubjectType: "", Count: 1},
	}, res)
}

func TestAccountTokenPlanStore_SummaryNsBound(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	newPlan := func(code, subjectType string, status types.TokenPlanStatus, nsUUIDs []string) database.AccountTokenPlan {
		return database.AccountTokenPlan{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:    code,
				PlanName:    code,
				SubjectType: database.NamespaceType(subjectType),
				Status:      status,
				CreatedBy:   "tester",
				Quotas:      []types.AccountTokenPlanModelQuota{},
				NsUUIDs:     nsUUIDs,
			},
		}
	}

	plans := []database.AccountTokenPlan{
		newPlan("ns-bound-user-draft", string(database.UserNamespace), types.TokenPlanStatusDraft, []string{"ns-1"}),
		newPlan("ns-bound-user-published", string(database.UserNamespace), types.TokenPlanStatusPublished, []string{"ns-1"}),
		newPlan("ns-unbound-user", string(database.UserNamespace), types.TokenPlanStatusDraft, []string{}),
		newPlan("ns-bound-org-published", string(database.OrgNamespace), types.TokenPlanStatusPublished, []string{"ns-2", "ns-3"}),
		newPlan("ns-unbound-org", string(database.OrgNamespace), types.TokenPlanStatusPublished, []string{}),
	}
	deleted := newPlan("ns-bound-deleted", string(database.UserNamespace), types.TokenPlanStatusDraft, []string{"ns-4"})
	deleted.DeletedAt = time.Now()
	plans = append(plans, deleted)

	for i := range plans {
		_, err := db.Core.NewInsert().Model(&plans[i]).Exec(ctx, &plans[i])
		require.NoError(t, err)
	}

	store := database.NewAccountTokenPlanStoreWithDB(db)
	res, err := store.SummaryNsBound(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.AcctTokenPlanNsBoundCount{
		{Status: types.TokenPlanStatusDraft, SubjectType: string(database.UserNamespace), Count: 1},
		{Status: types.TokenPlanStatusPublished, SubjectType: string(database.UserNamespace), Count: 1},
		{Status: types.TokenPlanStatusPublished, SubjectType: string(database.OrgNamespace), Count: 1},
	}, res)
}

func TestAccountTokenPlanStore_SummaryActiveSubscriptions(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	newPlan := func(code, subjectType string) database.AccountTokenPlan {
		return database.AccountTokenPlan{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:    code,
				PlanName:    code,
				SubjectType: database.NamespaceType(subjectType),
				CreatedBy:   "tester",
				Quotas:      []types.AccountTokenPlanModelQuota{},
				NsUUIDs:     []string{},
			},
		}
	}
	plans := []database.AccountTokenPlan{
		newPlan("active-sub-user", string(database.UserNamespace)),
		newPlan("active-sub-org", string(database.OrgNamespace)),
	}
	for i := range plans {
		_, err := db.Core.NewInsert().Model(&plans[i]).Exec(ctx, &plans[i])
		require.NoError(t, err)
	}

	newInstance := func(planID int64, status types.SubscriptionStatus) database.AccountTokenPlanInstance {
		return database.AccountTokenPlanInstance{
			NsUUID:             "ns-" + uuid.NewString(),
			PlanID:             planID,
			FirstPlanVersionID: 1,
			Status:             status,
			CreatedBy:          "tester",
		}
	}
	instances := []database.AccountTokenPlanInstance{
		newInstance(plans[0].ID, types.SubscriptionStatusActive),
		newInstance(plans[0].ID, types.SubscriptionStatusActive),
		newInstance(plans[0].ID, types.SubscriptionStatusCanceled),
		newInstance(plans[1].ID, types.SubscriptionStatusActive),
	}
	for i := range instances {
		_, err := db.Core.NewInsert().Model(&instances[i]).Exec(ctx, &instances[i])
		require.NoError(t, err)
	}

	store := database.NewAccountTokenPlanStoreWithDB(db)
	res, err := store.SummaryActiveSubscriptions(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.AcctTokenPlanActiveSubCount{
		{SubjectType: string(database.UserNamespace), Count: 2},
		{SubjectType: string(database.OrgNamespace), Count: 1},
	}, res)
}

func TestAccountTokenPlanStore_List_FillsActiveSubscriptionCounts(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	newPlan := func(code string) database.AccountTokenPlan {
		return database.AccountTokenPlan{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:  code,
				PlanName:  code,
				CreatedBy: "tester",
				Quotas:    []types.AccountTokenPlanModelQuota{},
				NsUUIDs:   []string{},
			},
		}
	}
	plans := []database.AccountTokenPlan{
		newPlan("list-active-plans-1"),
		newPlan("list-active-plans-2"),
	}
	for i := range plans {
		_, err := db.Core.NewInsert().Model(&plans[i]).Exec(ctx, &plans[i])
		require.NoError(t, err)
	}

	newInstance := func(planID int64, status types.SubscriptionStatus) database.AccountTokenPlanInstance {
		return database.AccountTokenPlanInstance{
			NsUUID:    "ns-" + uuid.NewString(),
			PlanID:    planID,
			Status:    status,
			CreatedBy: "tester",
		}
	}
	instances := []database.AccountTokenPlanInstance{
		newInstance(plans[0].ID, types.SubscriptionStatusActive),
		newInstance(plans[0].ID, types.SubscriptionStatusActive),
		newInstance(plans[0].ID, types.SubscriptionStatusCanceled),
		newInstance(plans[1].ID, types.SubscriptionStatusActive),
	}
	for i := range instances {
		_, err := db.Core.NewInsert().Model(&instances[i]).Exec(ctx, &instances[i])
		require.NoError(t, err)
	}

	store := database.NewAccountTokenPlanStoreWithDB(db)
	got, total, err := store.List(ctx, types.AcctTokenPlanListReq{Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, got, 2)

	counts := map[int64]int64{}
	for _, plan := range got {
		counts[plan.ID] = plan.ActiveSubscriptionCount
	}
	require.Equal(t, int64(2), counts[plans[0].ID])
	require.Equal(t, int64(1), counts[plans[1].ID])
}

func TestAccountTokenPlanStore_CancelInstance(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	t.Run("end at the end of the current period without refund", func(t *testing.T) {
		now := time.Now().Unix()
		nsUUID := "ns-cancel-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             nsUUID,
			PlanID:             11,
			FirstPlanVersionID: 1,
			OveragePolicy:      types.OveragePolicyPayAsYouGo,
			StartAt:            now - 40*tokenPlanDay,
			PeriodCount:        2,
			Status:             types.SubscriptionStatusActive,
			CreatedBy:          "tester",
		})
		// the first period already ended, the second one is the current period
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID, PlanVersionID: 1, PeriodNo: 1,
			StartAt: now - 40*tokenPlanDay, EndAt: now - 10*tokenPlanDay,
			Status: types.BillingStatusPaid, Reason: types.BillingReasonSubscriptionCreate,
		})
		currentEnd := now + 20*tokenPlanDay
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID, PlanVersionID: 1, PeriodNo: 2,
			StartAt: now - 10*tokenPlanDay, EndAt: currentEnd,
			Status: types.BillingStatusPaid, Reason: types.BillingReasonSubscriptionCycle,
		})

		canceled, err := store.CancelInstance(ctx, types.AcctTokenPlanCancelReq{
			NsUUID: nsUUID, PlanID: 11, InstanceID: instance.ID,
		})
		require.NoError(t, err)
		require.Equal(t, currentEnd, canceled.EndAt)

		var stored database.AccountTokenPlanInstance
		err = db.Core.NewSelect().Model(&stored).Where("id = ?", instance.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, currentEnd, stored.EndAt)
		// the instance still serves the quota of the paid period
		require.Equal(t, types.SubscriptionStatusActive, stored.Status)

		statementCount, err := db.Core.NewSelect().Model((*database.AccountStatement)(nil)).
			Where("user_uuid = ?", nsUUID).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, statementCount)
	})

	t.Run("cancel an instance whose period already ended", func(t *testing.T) {
		now := time.Now().Unix()
		nsUUID := "ns-cancel-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             nsUUID,
			PlanID:             12,
			FirstPlanVersionID: 1,
			Status:             types.SubscriptionStatusActive,
			CreatedBy:          "tester",
		})
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID, PlanVersionID: 1, PeriodNo: 1,
			StartAt: now - 30*tokenPlanDay, EndAt: now - tokenPlanDay,
			Status: types.BillingStatusPaid, Reason: types.BillingReasonSubscriptionCreate,
		})

		canceled, err := store.CancelInstance(ctx, types.AcctTokenPlanCancelReq{
			NsUUID: nsUUID, PlanID: 12, InstanceID: instance.ID,
		})
		require.NoError(t, err)
		require.Equal(t, now-tokenPlanDay, canceled.EndAt)

		// the cancellation takes effect on the next reconciliation
		result, err := store.GetActiveInstanceByNsUUID(ctx, types.AcctTokenPlanNSGetReq{
			NsUUID: nsUUID, RequestTime: time.Now(), EventUUID: uuid.New(),
		})
		require.NoError(t, err)
		require.Empty(t, result)

		var stored database.AccountTokenPlanInstance
		err = db.Core.NewSelect().Model(&stored).Where("id = ?", instance.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, types.SubscriptionStatusCanceled, stored.Status)
	})

	t.Run("reject an instance that does not match the namespace and plan", func(t *testing.T) {
		nsUUID := "ns-cancel-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             nsUUID,
			PlanID:             13,
			FirstPlanVersionID: 1,
			Status:             types.SubscriptionStatusActive,
			CreatedBy:          "tester",
		})

		_, err := store.CancelInstance(ctx, types.AcctTokenPlanCancelReq{
			NsUUID: nsUUID, PlanID: 14, InstanceID: instance.ID,
		})
		require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)

		_, err = store.CancelInstance(ctx, types.AcctTokenPlanCancelReq{
			NsUUID: "ns-other-" + uuid.NewString(), PlanID: 13, InstanceID: instance.ID,
		})
		require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
	})

	t.Run("no-op an instance that is not active", func(t *testing.T) {
		nsUUID := "ns-cancel-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             nsUUID,
			PlanID:             15,
			FirstPlanVersionID: 1,
			Status:             types.SubscriptionStatusCanceled,
			CreatedBy:          "tester",
		})

		canceled, err := store.CancelInstance(ctx, types.AcctTokenPlanCancelReq{
			NsUUID: nsUUID, PlanID: 15, InstanceID: instance.ID,
		})
		require.NoError(t, err)
		require.Equal(t, types.SubscriptionStatusCanceled, canceled.Status)
	})
}

func TestAccountTokenPlanStore_ListInstancesByNsUUID(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	nsUUID := "ns-history-" + uuid.NewString()
	first := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsUUID, PlanID: 31, Status: types.SubscriptionStatusCanceled, CreatedBy: "tester",
	})
	middle := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsUUID, PlanID: 32, Status: types.SubscriptionStatusActive, CreatedBy: "tester",
	})
	latest := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsUUID, PlanID: 33, Status: types.SubscriptionStatusExpired, CreatedBy: "tester",
	})
	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: "ns-history-" + uuid.NewString(), PlanID: 34, Status: types.SubscriptionStatusActive, CreatedBy: "tester",
	})

	page1, total, err := store.ListInstancesByNsUUID(ctx, types.AcctTokenPlanHistoryReq{
		NsUUID: nsUUID, Per: 2, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page1, 2)
	require.Equal(t, latest.ID, page1[0].ID)
	require.Equal(t, middle.ID, page1[1].ID)

	page2, total, err := store.ListInstancesByNsUUID(ctx, types.AcctTokenPlanHistoryReq{
		NsUUID: nsUUID, Per: 2, Page: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page2, 1)
	require.Equal(t, first.ID, page2[0].ID)
}

func TestAccountTokenPlanStore_ListPeriodsForNS(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:      "periods-plan",
			PlanName:      "Periods Plan",
			PriceAmount:   1200,
			Currency:      types.DEFAULT_CURRENCY,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  51,
		Version: 1,
	}
	_, err := db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()
	nsUUID := "ns-periods-" + uuid.NewString()
	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base,
		EndAt:              base + 90*tokenPlanDay,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})
	otherInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             "ns-periods-" + uuid.NewString(),
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base,
		EndAt:              base + 30*tokenPlanDay,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})

	newPeriod := func(instanceID int64, periodNo, startAt int64) database.AccountTokenPlanInstancePeriod {
		return insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID:    instanceID,
			PlanVersionID: version.ID,
			PeriodNo:      periodNo,
			StartAt:       startAt,
			EndAt:         startAt + 30*tokenPlanDay,
			Amount:        1200,
			CostAmount:    10.5,
			Status:        types.BillingStatusPaid,
			Reason:        types.BillingReasonSubscriptionCycle,
		})
	}
	first := newPeriod(instance.ID, 1, base)
	second := newPeriod(instance.ID, 2, base+30*tokenPlanDay)
	third := newPeriod(instance.ID, 3, base+60*tokenPlanDay)
	newPeriod(otherInstance.ID, 1, base)

	page1, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: nsUUID, Per: 2, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 3, page1.Total)
	require.Equal(t, int64(3600), page1.TotalAmount)
	require.Zero(t, page1.TotalCost)
	require.Len(t, page1.Periods, 2)
	require.Equal(t, third.ID, page1.Periods[0].ID)
	require.Equal(t, second.ID, page1.Periods[1].ID)
	require.NotNil(t, page1.Periods[0].PlanVersion)
	require.Equal(t, version.ID, page1.Periods[0].PlanVersion.ID)
	require.Equal(t, "periods-plan", page1.Periods[0].PlanVersion.PlanCode)

	page2, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: nsUUID, Per: 2, Page: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 3, page2.Total)
	require.Equal(t, int64(3600), page2.TotalAmount)
	require.Len(t, page2.Periods, 1)
	require.Equal(t, first.ID, page2.Periods[0].ID)

	// StartAt is inclusive and EndAt is exclusive.
	filtered, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID:  nsUUID,
		StartAt: base + 30*tokenPlanDay,
		EndAt:   base + 60*tokenPlanDay,
		Per:     20,
		Page:    1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, filtered.Total)
	require.Equal(t, int64(1200), filtered.TotalAmount)
	require.Zero(t, filtered.TotalCost)
	require.Len(t, filtered.Periods, 1)
	require.Equal(t, second.ID, filtered.Periods[0].ID)

	// A namespace only sees the periods of its own instances.
	other, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: otherInstance.NsUUID, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, other.Total)
	require.Equal(t, int64(1200), other.TotalAmount)
	require.Len(t, other.Periods, 1)
	require.Equal(t, otherInstance.ID, other.Periods[0].InstanceID)

	// A missing plan version snapshot does not hide the period: the period is
	// still returned, only without plan information.
	missingVersionInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             "ns-periods-" + uuid.NewString(),
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base,
		EndAt:              base + 30*tokenPlanDay,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})
	missingVersionPeriod := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    missingVersionInstance.ID,
		PlanVersionID: 1 << 62, // far beyond any id inserted by tests
		PeriodNo:      1,
		StartAt:       base,
		EndAt:         base + 30*tokenPlanDay,
		Amount:        1200,
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCycle,
	})

	orphaned, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: missingVersionInstance.NsUUID, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, orphaned.Total)
	require.Equal(t, int64(1200), orphaned.TotalAmount)
	require.Len(t, orphaned.Periods, 1)
	require.Equal(t, missingVersionPeriod.ID, orphaned.Periods[0].ID)
	require.Nil(t, orphaned.Periods[0].PlanVersion)

	// History does not depend on the instance lifecycle state: periods of
	// canceled and expired instances are still listed.
	closedNsUUID := "ns-periods-" + uuid.NewString()
	canceledInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             closedNsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base,
		EndAt:              base + 30*tokenPlanDay,
		Status:             types.SubscriptionStatusCanceled,
		CreatedBy:          "tester",
	})
	expiredInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             closedNsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base + 30*tokenPlanDay,
		EndAt:              base + 60*tokenPlanDay,
		Status:             types.SubscriptionStatusExpired,
		CreatedBy:          "tester",
	})
	canceledPeriod := newPeriod(canceledInstance.ID, 1, base)
	expiredPeriod := newPeriod(expiredInstance.ID, 1, base+30*tokenPlanDay)

	closed, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: closedNsUUID, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, closed.Total)
	require.Equal(t, int64(2400), closed.TotalAmount)
	require.Len(t, closed.Periods, 2)
	require.Equal(t, expiredPeriod.ID, closed.Periods[0].ID)
	require.Equal(t, canceledPeriod.ID, closed.Periods[1].ID)

	// The status filter narrows both the page and the summed amount.
	statusNsUUID := "ns-periods-" + uuid.NewString()
	statusInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             statusNsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		StartAt:            base,
		EndAt:              base + 60*tokenPlanDay,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})
	paidStatusPeriod := newPeriod(statusInstance.ID, 1, base)
	unpaidStatusPeriod := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    statusInstance.ID,
		PlanVersionID: version.ID,
		PeriodNo:      2,
		StartAt:       base + 30*tokenPlanDay,
		EndAt:         base + 60*tokenPlanDay,
		Amount:        500,
		Status:        types.BillingStatusUnpaid,
		Reason:        types.BillingReasonSubscriptionCycle,
	})

	allStatuses, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: statusNsUUID, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, allStatuses.Total)
	require.Equal(t, int64(1700), allStatuses.TotalAmount)

	paidOnly, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: statusNsUUID, Status: types.BillingStatusPaid, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, paidOnly.Total)
	require.Equal(t, int64(1200), paidOnly.TotalAmount)
	require.Len(t, paidOnly.Periods, 1)
	require.Equal(t, paidStatusPeriod.ID, paidOnly.Periods[0].ID)

	unpaidOnly, err := store.ListPeriodsForNS(ctx, types.AcctTokenPlanPeriodsFilter{
		NsUUID: statusNsUUID, Status: types.BillingStatusUnpaid, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, unpaidOnly.Total)
	require.Equal(t, int64(500), unpaidOnly.TotalAmount)
	require.Len(t, unpaidOnly.Periods, 1)
	require.Equal(t, unpaidStatusPeriod.ID, unpaidOnly.Periods[0].ID)
}

func TestAccountTokenPlanStore_ListPeriodsForInstance(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:      "instance-periods-plan",
			PlanName:      "Instance Periods Plan",
			PriceAmount:   1200,
			Currency:      types.DEFAULT_CURRENCY,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  61,
		Version: 1,
	}
	_, err := db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()
	newInstance := func() database.AccountTokenPlanInstance {
		return insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             "ns-instance-periods-" + uuid.NewString(),
			PlanID:             version.PlanID,
			FirstPlanVersionID: version.ID,
			StartAt:            base,
			EndAt:              base + 90*tokenPlanDay,
			Status:             types.SubscriptionStatusActive,
			CreatedBy:          "tester",
		})
	}
	instance := newInstance()
	otherInstance := newInstance()

	newPeriod := func(instanceID, periodNo, startAt, amount int64, cost float64, status types.BillingStatus) database.AccountTokenPlanInstancePeriod {
		return insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID:    instanceID,
			PlanVersionID: version.ID,
			PeriodNo:      periodNo,
			StartAt:       startAt,
			EndAt:         startAt + 30*tokenPlanDay,
			Amount:        amount,
			CostAmount:    cost,
			Status:        status,
			Reason:        types.BillingReasonSubscriptionCycle,
		})
	}
	first := newPeriod(instance.ID, 1, base, 1200, 300.5, types.BillingStatusPaid)
	second := newPeriod(instance.ID, 2, base+30*tokenPlanDay, 1200, 400.25, types.BillingStatusPaid)
	third := newPeriod(instance.ID, 3, base+60*tokenPlanDay, 500, 100.75, types.BillingStatusUnpaid)
	newPeriod(otherInstance.ID, 1, base, 1200, 50, types.BillingStatusPaid)

	t.Run("Lists one instance's periods highest period first", func(t *testing.T) {
		page1, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: instance.ID, Per: 2, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 3, page1.Total)
		require.Equal(t, int64(2900), page1.TotalAmount)
		require.InDelta(t, 801.5, page1.TotalCost, 1e-9)
		require.Len(t, page1.Periods, 2)
		require.Equal(t, third.ID, page1.Periods[0].ID)
		require.Equal(t, second.ID, page1.Periods[1].ID)
		require.NotNil(t, page1.Periods[0].PlanVersion)
		require.Equal(t, version.ID, page1.Periods[0].PlanVersion.ID)
		require.Equal(t, "instance-periods-plan", page1.Periods[0].PlanVersion.PlanCode)

		page2, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: instance.ID, Per: 2, Page: 2,
		})
		require.NoError(t, err)
		require.Equal(t, 3, page2.Total)
		require.Equal(t, int64(2900), page2.TotalAmount)
		require.InDelta(t, 801.5, page2.TotalCost, 1e-9)
		require.Len(t, page2.Periods, 1)
		require.Equal(t, first.ID, page2.Periods[0].ID)
	})

	t.Run("Date range bounds the period start time", func(t *testing.T) {
		// StartAt is inclusive and EndAt is exclusive.
		filtered, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: instance.ID,
			StartAt:    base + 30*tokenPlanDay,
			EndAt:      base + 60*tokenPlanDay,
			Per:        20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, filtered.Total)
		require.Equal(t, int64(1200), filtered.TotalAmount)
		require.InDelta(t, 400.25, filtered.TotalCost, 1e-9)
		require.Len(t, filtered.Periods, 1)
		require.Equal(t, second.ID, filtered.Periods[0].ID)
	})

	t.Run("Status filter narrows the page and the totals", func(t *testing.T) {
		paidOnly, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: instance.ID, Status: types.BillingStatusPaid, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, paidOnly.Total)
		require.Equal(t, int64(2400), paidOnly.TotalAmount)
		require.InDelta(t, 700.75, paidOnly.TotalCost, 1e-9)
		require.Len(t, paidOnly.Periods, 2)
		require.Equal(t, second.ID, paidOnly.Periods[0].ID)
		require.Equal(t, first.ID, paidOnly.Periods[1].ID)

		unpaidOnly, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: instance.ID, Status: types.BillingStatusUnpaid, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, unpaidOnly.Total)
		require.Equal(t, int64(500), unpaidOnly.TotalAmount)
		require.InDelta(t, 100.75, unpaidOnly.TotalCost, 1e-9)
		require.Len(t, unpaidOnly.Periods, 1)
		require.Equal(t, third.ID, unpaidOnly.Periods[0].ID)
	})

	t.Run("Only the requested instance's periods are returned", func(t *testing.T) {
		other, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: otherInstance.ID, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, other.Total)
		require.Equal(t, int64(1200), other.TotalAmount)
		require.InDelta(t, 50.0, other.TotalCost, 1e-9)
		require.Len(t, other.Periods, 1)
		require.Equal(t, otherInstance.ID, other.Periods[0].InstanceID)
	})

	t.Run("Unknown instance yields an empty result", func(t *testing.T) {
		empty, err := store.ListPeriodsForInstance(ctx, types.AcctTokenPlanInstancePeriodsFilter{
			InstanceID: 1 << 62, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, empty.Total)
		require.Equal(t, int64(0), empty.TotalAmount)
		require.Zero(t, empty.TotalCost)
		require.Empty(t, empty.Periods)
	})
}

func TestAccountTokenPlanStore_UpdateInstanceOveragePolicy(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	t.Run("updates the active instance overage policy", func(t *testing.T) {
		now := time.Now().Unix()
		nsUUID := "ns-update-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:             nsUUID,
			PlanID:             21,
			FirstPlanVersionID: 1,
			OveragePolicy:      types.OveragePolicyPayAsYouGo,
			StartAt:            now,
			EndAt:              now + tokenPlanDay,
			Status:             types.SubscriptionStatusActive,
			CreatedBy:          "tester",
		})

		updated, err := store.UpdateInstanceOveragePolicy(ctx, types.AcctTokenPlanUpdateNSReq{
			OveragePolicy: types.OveragePolicyReject,
			NsUUID:        nsUUID,
			RequestTime:   time.Now(),
			EventUUID:     uuid.New(),
			CurrentUser:   "tester",
		})
		require.NoError(t, err)
		require.Equal(t, types.OveragePolicyReject, updated.OveragePolicy)

		var stored database.AccountTokenPlanInstance
		err = db.Core.NewSelect().Model(&stored).Where("id = ?", instance.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, types.OveragePolicyReject, stored.OveragePolicy)
	})

	t.Run("rejects a namespace without an active instance", func(t *testing.T) {
		_, err := store.UpdateInstanceOveragePolicy(ctx, types.AcctTokenPlanUpdateNSReq{
			OveragePolicy: types.OveragePolicyReject,
			NsUUID:        "ns-update-" + uuid.NewString(),
			RequestTime:   time.Now(),
			EventUUID:     uuid.New(),
		})
		require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
	})
}

func TestAccountTokenPlanStore_ListActiveTokenPlanNamespaces(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	nsActiveA := "ns-active-" + uuid.NewString()
	nsActiveB := "ns-active-" + uuid.NewString()
	nsCanceled := "ns-canceled-" + uuid.NewString()
	nsExpired := "ns-expired-" + uuid.NewString()

	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsActiveA, PlanID: 41, Status: types.SubscriptionStatusActive, CreatedBy: "tester",
	})
	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsActiveA, PlanID: 42, Status: types.SubscriptionStatusActive, CreatedBy: "tester",
	})
	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsActiveB, PlanID: 43, Status: types.SubscriptionStatusActive, CreatedBy: "tester",
	})
	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsCanceled, PlanID: 44, Status: types.SubscriptionStatusCanceled, CreatedBy: "tester",
	})
	insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID: nsExpired, PlanID: 45, Status: types.SubscriptionStatusExpired, CreatedBy: "tester",
	})

	store := database.NewAccountTokenPlanStoreWithDB(db)
	namespaces, err := store.ListActiveTokenPlanNamespaces(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{nsActiveA, nsActiveB}, namespaces)
}

func TestAccountTokenPlanStore_UpgradeInstance(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	now := time.Now()
	nsUUID := "ns-upgrade-" + uuid.NewString()
	eventUUID := uuid.New()

	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: nsUUID, Balance: 5000,
	}).Exec(ctx)
	require.NoError(t, err)

	oldPlan := database.AccountTokenPlan{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:      "plan-current-" + uuid.NewString(),
			PlanName:      "Current Plan",
			PriceAmount:   1000,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
			Status:        types.TokenPlanStatusPublished,
			CreatedBy:     "tester",
		},
	}
	_, err = db.Core.NewInsert().Model(&oldPlan).Exec(ctx, &oldPlan)
	require.NoError(t, err)

	oldInstance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             oldPlan.ID,
		FirstPlanVersionID: 1,
		OveragePolicy:      types.OveragePolicyPayAsYouGo,
		StartAt:            now.Add(-10 * 24 * time.Hour).Unix(),
		PeriodCount:        1,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})
	oldPeriod := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    oldInstance.ID,
		PlanVersionID: 1,
		PeriodNo:      1,
		StartAt:       now.Add(-10 * 24 * time.Hour).Unix(),
		EndAt:         now.Add(10 * 24 * time.Hour).Unix(),
		Amount:        1000,
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCreate,
	})
	for _, quota := range []database.AccountTokenPlanPeriodModelQuota{
		{PeriodID: oldPeriod.ID, ModelID: "m1", QuotaTokens: 1000, UsedTokens: 500},
		{PeriodID: oldPeriod.ID, ModelID: "m2", QuotaTokens: 1000, UsedTokens: 0},
	} {
		_, err = db.Core.NewInsert().Model(&quota).Exec(ctx, &quota)
		require.NoError(t, err)
	}

	targetPlan := database.AccountTokenPlan{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:      "plan-upgrade-" + uuid.NewString(),
			PlanName:      "Upgrade Plan",
			PriceAmount:   2000,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
			Status:        types.TokenPlanStatusPublished,
			CreatedBy:     "tester",
			Quotas: []types.AccountTokenPlanModelQuota{
				{ModelID: "new-model", QuotaTokens: 5000},
			},
		},
	}
	_, err = db.Core.NewInsert().Model(&targetPlan).Exec(ctx, &targetPlan)
	require.NoError(t, err)

	targetVersion := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:      targetPlan.PlanCode,
			PlanName:      targetPlan.PlanName,
			PriceAmount:   2000,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
			Status:        types.TokenPlanStatusPublished,
			CreatedBy:     "tester",
			Quotas: []types.AccountTokenPlanModelQuota{
				{ModelID: "new-model", QuotaTokens: 5000},
			},
		},
		PlanID:  targetPlan.ID,
		Version: 1,
	}
	_, err = db.Core.NewInsert().Model(&targetVersion).Exec(ctx, &targetVersion)
	require.NoError(t, err)

	upgraded, err := store.UpgradeInstance(ctx, types.AcctTokenPlanUpgradeReq{
		NsUUID:        nsUUID,
		InstanceID:    oldInstance.ID,
		PlanID:        targetPlan.ID,
		OveragePolicy: types.OveragePolicyReject,
		CurrentUser:   "tester",
		RequestTime:   now,
		EventUUID:     eventUUID,
	})
	require.NoError(t, err)
	require.Equal(t, targetPlan.ID, upgraded.PlanID)
	require.Equal(t, types.SubscriptionStatusActive, upgraded.Status)
	require.Equal(t, int64(1), upgraded.PeriodCount)
	require.NotNil(t, upgraded.LatestPeriod)
	require.Equal(t, int64(1250), upgraded.LatestPeriod.Amount)
	require.Equal(t, types.BillingReasionSubscriptionUpgrade, upgraded.LatestPeriod.Reason)

	var storedOld database.AccountTokenPlanInstance
	err = db.Core.NewSelect().Model(&storedOld).Where("id = ?", oldInstance.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, types.SubscriptionStatusCanceled, storedOld.Status)
	require.Equal(t, now.Unix(), storedOld.ExpiredAt)

	var storedOldPeriod database.AccountTokenPlanInstancePeriod
	err = db.Core.NewSelect().Model(&storedOldPeriod).Where("id = ?", oldPeriod.ID).Scan(ctx)
	require.NoError(t, err)

	var stmt database.AccountStatement
	err = db.Core.NewSelect().Model(&stmt).
		Where("user_uuid = ?", nsUUID).
		Where("scene = ?", types.SceneTokenPlan).
		Where("token_plan_period_id = ?", upgraded.LatestPeriod.ID).
		Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, float64(-1250), stmt.Value)
}

func TestAccountTokenPlanStore_GetActiveSubscription(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	t.Run("no active subscription", func(t *testing.T) {
		sub, err := store.GetActiveSubscription(ctx, "ns-no-subscription-"+uuid.NewString())
		require.NoError(t, err)
		require.Nil(t, sub)
	})

	t.Run("returns the active instance policy", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-active-" + uuid.NewString(),
			PlanID:        61,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})

		sub, err := store.GetActiveSubscription(ctx, instance.NsUUID)
		require.NoError(t, err)
		require.NotNil(t, sub)
		require.Equal(t, instance.ID, sub.InstanceID)
		require.Equal(t, types.OveragePolicyReject, sub.OveragePolicy)
	})

	t.Run("ignores inactive instances", func(t *testing.T) {
		nsUUID := "ns-inactive-" + uuid.NewString()
		active := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        nsUUID,
			PlanID:        62,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})
		// Inserted after the active one, so an unfiltered newest-first lookup
		// would return the expired instance instead.
		insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        nsUUID,
			PlanID:        63,
			OveragePolicy: types.OveragePolicyPayAsYouGo,
			Status:        types.SubscriptionStatusExpired,
			CreatedBy:     "tester",
		})

		sub, err := store.GetActiveSubscription(ctx, nsUUID)
		require.NoError(t, err)
		require.NotNil(t, sub)
		require.Equal(t, active.ID, sub.InstanceID)
		require.Equal(t, types.OveragePolicyReject, sub.OveragePolicy)
	})
}

func TestAccountTokenPlanStore_GetLatestPeriodModelQuota(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)
	now := time.Now().Unix()

	insertPeriod := func(t *testing.T, instanceID, periodNo, startAt, endAt int64) database.AccountTokenPlanInstancePeriod {
		return insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instanceID,
			PeriodNo:   periodNo,
			StartAt:    startAt,
			EndAt:      endAt,
			Status:     types.BillingStatusPaid,
			Reason:     types.BillingReasonSubscriptionCycle,
		})
	}
	insertQuota := func(t *testing.T, periodID int64, modelID string, usedTokens, quotaTokens int64) {
		_, err := db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
			PeriodID:    periodID,
			ModelID:     modelID,
			TaskType:    "text-generation",
			QuotaTokens: quotaTokens,
			UsedTokens:  usedTokens,
		}).Exec(ctx)
		require.NoError(t, err)
	}

	t.Run("no period yet", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-no-period-" + uuid.NewString(),
			PlanID:        71,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})

		got, err := store.GetLatestPeriodModelQuota(ctx, instance.ID, "model-a")
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("model not covered", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-not-covered-" + uuid.NewString(),
			PlanID:        72,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})
		period := insertPeriod(t, instance.ID, 1, now-tokenPlanDay, now+tokenPlanDay)
		insertQuota(t, period.ID, "model-a", 0, 1000)

		got, err := store.GetLatestPeriodModelQuota(ctx, instance.ID, "model-b")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, period.ID, got.PeriodID)
		require.Nil(t, got.Quota)
	})

	t.Run("returns the latest period model quota", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-quota-" + uuid.NewString(),
			PlanID:        73,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})
		period := insertPeriod(t, instance.ID, 1, now-tokenPlanDay, now+10*tokenPlanDay)
		_, err := db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
			PeriodID:          period.ID,
			ModelID:           "model-a",
			TaskType:          "text-generation",
			QuotaTokens:       1000,
			UsedTokens:        120,
			QuotaRequestCount: 10,
			UsedRequestCount:  5,
		}).Exec(ctx)
		require.NoError(t, err)

		got, err := store.GetLatestPeriodModelQuota(ctx, instance.ID, "model-a")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, period.ID, got.PeriodID)
		require.Equal(t, period.StartAt, got.StartAt)
		require.Equal(t, period.EndAt, got.EndAt)
		require.NotNil(t, got.Quota)
		require.Equal(t, "model-a", got.Quota.ModelID)
		require.Equal(t, int64(120), got.Quota.UsedTokens)
		require.Equal(t, int64(1000), got.Quota.QuotaTokens)
		require.Equal(t, int64(5), got.Quota.UsedRequestCount)
		require.Equal(t, int64(10), got.Quota.QuotaRequestCount)
	})

	t.Run("uses the latest period", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-latest-period-" + uuid.NewString(),
			PlanID:        74,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})
		older := insertPeriod(t, instance.ID, 1, now-20*tokenPlanDay, now-10*tokenPlanDay)
		newer := insertPeriod(t, instance.ID, 2, now-10*tokenPlanDay, now+20*tokenPlanDay)
		insertQuota(t, older.ID, "model-a", 10, 1000)
		insertQuota(t, newer.ID, "model-a", 900, 1000)

		got, err := store.GetLatestPeriodModelQuota(ctx, instance.ID, "model-a")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, newer.ID, got.PeriodID)
		require.NotNil(t, got.Quota)
		require.Equal(t, int64(900), got.Quota.UsedTokens)
	})

	t.Run("latest period without a quota does not fall back to an older period", func(t *testing.T) {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:        "ns-latest-uncovered-" + uuid.NewString(),
			PlanID:        75,
			OveragePolicy: types.OveragePolicyReject,
			Status:        types.SubscriptionStatusActive,
			CreatedBy:     "tester",
		})
		older := insertPeriod(t, instance.ID, 1, now-20*tokenPlanDay, now-10*tokenPlanDay)
		newer := insertPeriod(t, instance.ID, 2, now-10*tokenPlanDay, now+20*tokenPlanDay)
		// The model was covered in the previous period but not in the current
		// one; the lookup must return the current period with no quota.
		insertQuota(t, older.ID, "model-a", 900, 1000)
		insertQuota(t, newer.ID, "model-b", 0, 1000)

		got, err := store.GetLatestPeriodModelQuota(ctx, instance.ID, "model-a")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, newer.ID, got.PeriodID)
		require.Nil(t, got.Quota)
	})
}

func TestAccountTokenPlanStore_GetPeriodByEvent(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAccountTokenPlanStoreWithDB(db)
	now := time.Now()

	getPeriod := func(nsUUID string, eventTime time.Time) (*database.AccountTokenPlanInstancePeriod, error) {
		return store.GetPeriodByEvent(ctx, types.AcctTokenPlanNSEventReq{
			NsUUID: nsUUID,
			// reconcile as of "now" and select the covering period by the
			// event time, so a late event still rolls the subscription forward
			// and is attributed to its historical period
			RequestTime: time.Now(),
			EventTime:   eventTime,
			EventUUID:   uuid.New(),
			CurrentUser: "tester",
		})
	}

	t.Run("namespace without subscription has no coverage", func(t *testing.T) {
		got, err := getPeriod("ns-event-none-"+uuid.NewString(), now)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("paid period covering the event time is returned", func(t *testing.T) {
		nsUUID := "ns-event-cover-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    51,
			Status:    types.SubscriptionStatusActive,
			CreatedBy: "tester",
		})
		want := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID,
			PeriodNo:   1,
			StartAt:    now.Add(-24 * time.Hour).Unix(),
			EndAt:      now.Add(24 * time.Hour).Unix(),
			Status:     types.BillingStatusPaid,
			Reason:     types.BillingReasonSubscriptionCreate,
		})

		got, err := getPeriod(nsUUID, now)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, want.ID, got.ID)
		require.Equal(t, types.BillingStatusPaid, got.Status)
	})

	t.Run("period start boundary is inclusive", func(t *testing.T) {
		nsUUID := "ns-event-start-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    52,
			Status:    types.SubscriptionStatusActive,
			CreatedBy: "tester",
		})
		start := now.Add(-time.Hour).Truncate(time.Second)
		want := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID,
			PeriodNo:   1,
			StartAt:    start.Unix(),
			EndAt:      start.Add(48 * time.Hour).Unix(),
			Status:     types.BillingStatusPaid,
			Reason:     types.BillingReasonSubscriptionCreate,
		})

		got, err := getPeriod(nsUUID, start)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, want.ID, got.ID)
	})

	t.Run("unpaid period still covers its window and the retry failure does not fail the lookup", func(t *testing.T) {
		nsUUID := "ns-event-unpaid-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    53,
			Status:    types.SubscriptionStatusActive,
			CreatedBy: "tester",
		})
		want := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID,
			PeriodNo:   1,
			StartAt:    now.Add(-24 * time.Hour).Unix(),
			EndAt:      now.Add(24 * time.Hour).Unix(),
			Amount:     100,
			Status:     types.BillingStatusUnpaid,
			Reason:     types.BillingReasonDeductFailed,
		})

		// the reconcile retries the failed fee deduction (which fails again
		// without an account) but with auto-scan semantics that must not fail
		// the lookup, and the window match ignores the unpaid status
		got, err := getPeriod(nsUUID, now)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, want.ID, got.ID)
	})

	t.Run("period that has not started grants no coverage", func(t *testing.T) {
		nsUUID := "ns-event-future-" + uuid.NewString()
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    54,
			Status:    types.SubscriptionStatusActive,
			CreatedBy: "tester",
		})
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID,
			PeriodNo:   1,
			StartAt:    now.Add(time.Hour).Unix(),
			EndAt:      now.Add(48 * time.Hour).Unix(),
			Status:     types.BillingStatusPaid,
			Reason:     types.BillingReasonSubscriptionCreate,
		})

		got, err := getPeriod(nsUUID, now)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("canceled subscription keeps coverage until the window ends", func(t *testing.T) {
		nsUUID := "ns-event-canceled-" + uuid.NewString()
		end := now.Add(24 * time.Hour).Truncate(time.Second)
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    55,
			EndAt:     end.Unix(),
			Status:    types.SubscriptionStatusCanceled,
			CreatedBy: "tester",
		})
		want := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID,
			PeriodNo:   1,
			StartAt:    now.Add(-24 * time.Hour).Unix(),
			EndAt:      end.Unix(),
			Status:     types.BillingStatusPaid,
			Reason:     types.BillingReasonSubscriptionCreate,
		})

		got, err := getPeriod(nsUUID, now)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, want.ID, got.ID)

		// the period end boundary is exclusive: coverage died with the window
		got, err = getPeriod(nsUUID, end)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

func TestAccountTokenPlanStore_GetPeriodByEvent_RollsEndedPeriodBeforeSelecting(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	now := time.Now()
	nsUUID := "ns-event-roll-" + uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: nsUUID}).Exec(ctx)
	require.NoError(t, err)

	// free plan: the renewal period fee is zero, so the rolled period is paid
	version := database.AccountTokenPlanVersion{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
		},
		PlanID:  520,
		Version: 1,
	}
	_, err = db.Core.NewInsert().Model(&version).Exec(ctx, &version)
	require.NoError(t, err)

	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:             nsUUID,
		PlanID:             version.PlanID,
		FirstPlanVersionID: version.ID,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          "tester",
	})

	prevStart := now.Add(-31 * 24 * time.Hour)
	prevEnd := now.Add(-time.Minute)
	insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID:    instance.ID,
		PlanVersionID: version.ID,
		PeriodNo:      1,
		StartAt:       prevStart.Unix(),
		EndAt:         prevEnd.Unix(),
		Status:        types.BillingStatusPaid,
		Reason:        types.BillingReasonSubscriptionCreate,
	})

	store := database.NewAccountTokenPlanStoreWithDB(db)
	// an event arriving just after the period window closed must not lose
	// coverage: the lookup reconciles first, chaining a renewal period from the
	// previous window end, and that rolled period then covers the event
	got, err := store.GetPeriodByEvent(ctx, types.AcctTokenPlanNSEventReq{
		NsUUID:      nsUUID,
		RequestTime: now,
		EventTime:   now,
		EventUUID:   uuid.New(),
		CurrentUser: "tester",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(2), got.PeriodNo)
	require.Equal(t, prevEnd.Unix(), got.StartAt)
	require.Equal(t, types.BillingStatusPaid, got.Status)
}

func TestAccountTokenPlanStore_GetPeriodByEvent_NewestPeriodWinsOnOverlap(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	now := time.Now()
	nsUUID := "ns-event-overlap-" + uuid.NewString()

	// an upgrade leaves the previous canceled instance's paid period window
	// overlapping the new instance's first period
	prev := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:    nsUUID,
		PlanID:    530,
		EndAt:     now.Add(24 * time.Hour).Unix(),
		Status:    types.SubscriptionStatusCanceled,
		CreatedBy: "tester",
	})
	insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID: prev.ID,
		PeriodNo:   1,
		StartAt:    now.Add(-24 * time.Hour).Unix(),
		EndAt:      now.Add(24 * time.Hour).Unix(),
		Status:     types.BillingStatusPaid,
		Reason:     types.BillingReasonSubscriptionCreate,
	})

	next := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:    nsUUID,
		PlanID:    531,
		Status:    types.SubscriptionStatusActive,
		CreatedBy: "tester",
	})
	want := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID: next.ID,
		PeriodNo:   1,
		StartAt:    now.Add(-time.Hour).Unix(),
		EndAt:      now.Add(23 * time.Hour).Unix(),
		Status:     types.BillingStatusPaid,
		Reason:     types.BillingReasonSubscriptionCreate,
	})

	store := database.NewAccountTokenPlanStoreWithDB(db)
	got, err := store.GetPeriodByEvent(ctx, types.AcctTokenPlanNSEventReq{
		NsUUID:      nsUUID,
		RequestTime: now,
		EventTime:   now,
		EventUUID:   uuid.New(),
		CurrentUser: "tester",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, want.ID, got.ID)
}

func insertTokenPlanInstance(ctx context.Context, t *testing.T, db *database.DB,
	instance database.AccountTokenPlanInstance) database.AccountTokenPlanInstance {
	t.Helper()

	_, err := db.Core.NewInsert().Model(&instance).Exec(ctx, &instance)
	require.NoError(t, err)
	return instance
}

// insertTokenPlan inserts a plan with an explicit id so a test can bind
// instances, which only reference a plan by id, to a known plan code regardless
// of the ids earlier tests in this package have already consumed. Pass
// autoIncrementPlanID to let the database assign the id.
func insertTokenPlan(ctx context.Context, t *testing.T, db *database.DB,
	id int64, planCode, planName string, priceAmount int64) database.AccountTokenPlan {
	t.Helper()

	plan := database.AccountTokenPlan{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			ID:            id,
			PlanCode:      planCode,
			PlanName:      planName,
			PriceAmount:   priceAmount,
			Currency:      types.DEFAULT_CURRENCY,
			BillingPeriod: 1,
			PeriodUnit:    types.UnitMonth,
			CreatedBy:     "user-1",
			Quotas:        []types.AccountTokenPlanModelQuota{},
			NsUUIDs:       []string{},
		},
	}
	_, err := db.Core.NewInsert().Model(&plan).Exec(ctx, &plan)
	require.NoError(t, err)
	return plan
}

// autoIncrementPlanID lets insertTokenPlan leave the plan id to the database.
const autoIncrementPlanID int64 = 0

func insertTokenPlanPeriod(ctx context.Context, t *testing.T, db *database.DB,
	period database.AccountTokenPlanInstancePeriod) database.AccountTokenPlanInstancePeriod {
	t.Helper()

	_, err := db.Core.NewInsert().Model(&period).Exec(ctx, &period)
	require.NoError(t, err)
	return period
}

func TestAccountTokenPlanStore_GetByPlanName(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	plans := []database.AccountTokenPlan{
		{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:  "gbpn-001",
				PlanName:  "Basic Plan",
				CreatedBy: "tester",
				Quotas:    []types.AccountTokenPlanModelQuota{},
				NsUUIDs:   []string{},
			},
		},
		{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode:  "gbpn-002",
				PlanName:  "Deleted Plan",
				CreatedBy: "tester",
				Quotas:    []types.AccountTokenPlanModelQuota{},
				NsUUIDs:   []string{},
			},
			DeletedAt: time.Now(),
		},
	}
	for i := range plans {
		_, err := db.Core.NewInsert().Model(&plans[i]).Exec(ctx, &plans[i])
		require.NoError(t, err)
	}

	store := database.NewAccountTokenPlanStoreWithDB(db)

	found, err := store.GetByPlanName(ctx, "Basic Plan")
	require.NoError(t, err)
	require.Equal(t, "gbpn-001", found.PlanCode)

	// Soft-deleted plans are excluded from the duplicate-name check so a deleted
	// plan name can be reused.
	_, err = store.GetByPlanName(ctx, "Deleted Plan")
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)

	_, err = store.GetByPlanName(ctx, "Missing Plan")
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
}

// testTokenPlan builds a minimal token plan row for store tests.
func testTokenPlan(code, name string) database.AccountTokenPlan {
	return database.AccountTokenPlan{
		AccountTokenPlanCommon: database.AccountTokenPlanCommon{
			PlanCode:  code,
			PlanName:  name,
			CreatedBy: "tester",
			Quotas:    []types.AccountTokenPlanModelQuota{},
			NsUUIDs:   []string{},
		},
	}
}

func TestAccountTokenPlanStore_CreateRejectsDuplicateName(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAccountTokenPlanStoreWithDB(db)

	first, err := store.Create(ctx, testTokenPlan("dup-name-1", "shared-name"))
	require.NoError(t, err)
	require.NotZero(t, first.ID)

	_, err = store.Create(ctx, testTokenPlan("dup-name-2", "shared-name"))
	require.ErrorIs(t, err, errorx.ErrTokenPlanNameDuplicate)

	// A soft-deleted plan no longer reserves its name.
	require.NoError(t, store.Delete(ctx, first.ID))
	reused, err := store.Create(ctx, testTokenPlan("dup-name-3", "shared-name"))
	require.NoError(t, err)
	require.NotZero(t, reused.ID)
}

func TestAccountTokenPlanStore_UpdateRejectsDuplicateName(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAccountTokenPlanStoreWithDB(db)

	_, err := store.Create(ctx, testTokenPlan("upd-name-1", "name-one"))
	require.NoError(t, err)
	second, err := store.Create(ctx, testTokenPlan("upd-name-2", "name-two"))
	require.NoError(t, err)

	// Keeping the current name never conflicts with the plan itself.
	second.PlanName = "name-two"
	second.PriceAmount = 100
	_, err = store.Update(ctx, *second)
	require.NoError(t, err)

	// Taking another active plan's name is rejected.
	second.PlanName = "name-one"
	_, err = store.Update(ctx, *second)
	require.ErrorIs(t, err, errorx.ErrTokenPlanNameDuplicate)
}

// openTokenPlanConcurrencyDB returns a real pool-backed handle. InitTestDB
// funnels every query through one backend session, so cross-session advisory
// locks are invisible to it; concurrency tests need independent connections.
func openTokenPlanConcurrencyDB(t *testing.T) *database.DB {
	t.Helper()

	tests.InitTestDB()
	realDB, err := database.NewDB(context.Background(), database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     tests.TestDBDSN() + "sslmode=disable",
	})
	require.NoError(t, err)
	return realDB
}

func TestAccountTokenPlanStore_CreateConcurrentSameName(t *testing.T) {
	ctx := context.Background()
	realDB := openTokenPlanConcurrencyDB(t)
	defer realDB.Close()

	store := database.NewAccountTokenPlanStoreWithDB(realDB)
	base := time.Now().UnixNano()
	name := fmt.Sprintf("concurrent-create-%d", base)
	defer func() {
		if _, err := realDB.Core.NewRaw("DELETE FROM account_token_plans WHERE plan_name = ?", name).Exec(ctx); err != nil {
			t.Errorf("clean up concurrent create plans: %v", err)
		}
	}()

	const attempts = 5
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := store.Create(ctx, testTokenPlan(fmt.Sprintf("cc-%d-%d", base, i), name))
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	var succeeded, rejected int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, errorx.ErrTokenPlanNameDuplicate):
			rejected++
		default:
			t.Fatalf("unexpected concurrent create error: %v", err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, attempts-1, rejected)

	count, err := realDB.Core.NewSelect().Model((*database.AccountTokenPlan)(nil)).Where("plan_name = ?", name).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestAccountTokenPlanStore_UpdateConcurrentSameName(t *testing.T) {
	ctx := context.Background()
	realDB := openTokenPlanConcurrencyDB(t)
	defer realDB.Close()

	store := database.NewAccountTokenPlanStoreWithDB(realDB)
	base := time.Now().UnixNano()
	target := fmt.Sprintf("concurrent-rename-%d", base)

	plans := make([]*database.AccountTokenPlan, 2)
	for i := range plans {
		created, err := store.Create(ctx, testTokenPlan(
			fmt.Sprintf("cr-%d-%d", base, i),
			fmt.Sprintf("rename-src-%d-%d", base, i),
		))
		require.NoError(t, err)
		plans[i] = created
	}
	defer func() {
		for _, plan := range plans {
			if _, err := realDB.Core.NewRaw("DELETE FROM account_token_plans WHERE id = ?", plan.ID).Exec(ctx); err != nil {
				t.Errorf("clean up concurrent rename plan %d: %v", plan.ID, err)
			}
		}
	}()

	start := make(chan struct{})
	results := make(chan error, len(plans))
	var wg sync.WaitGroup
	for i := range plans {
		wg.Add(1)
		go func(plan *database.AccountTokenPlan) {
			defer wg.Done()
			<-start
			updated := *plan
			updated.PlanName = target
			_, err := store.Update(ctx, updated)
			results <- err
		}(plans[i])
	}
	close(start)
	wg.Wait()
	close(results)

	var succeeded, rejected int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, errorx.ErrTokenPlanNameDuplicate):
			rejected++
		default:
			t.Fatalf("unexpected concurrent update error: %v", err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, len(plans)-1, rejected)

	count, err := realDB.Core.NewSelect().Model((*database.AccountTokenPlan)(nil)).Where("plan_name = ?", target).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestAccountTokenPlanStore_ListInstances(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	base := time.Now().Add(-30 * 24 * time.Hour).Truncate(time.Second)
	// created_at defaults to the transaction timestamp, which is identical for
	// every row inserted in one transaction, so it is set explicitly to give the
	// date range filter distinct bounds to work with.
	newInstance := func(nsUUID string, planID int64, status types.SubscriptionStatus, createdAt time.Time) database.AccountTokenPlanInstance {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    nsUUID,
			PlanID:    planID,
			Status:    status,
			StartAt:   createdAt.Unix(),
			EndAt:     createdAt.Add(24 * time.Hour).Unix(),
			CreatedBy: "user-1",
		})
		_, err := db.Core.NewUpdate().
			Model((*database.AccountTokenPlanInstance)(nil)).
			Set("created_at = ?", createdAt).
			Where("id = ?", instance.ID).
			Exec(ctx)
		require.NoError(t, err)
		return instance
	}
	older := newInstance("ns-1", 1, types.SubscriptionStatusActive, base)
	middle := newInstance("ns-1", 2, types.SubscriptionStatusExpired, base.Add(24*time.Hour))
	newer := newInstance("ns-2", 1, types.SubscriptionStatusCanceled, base.Add(48*time.Hour))
	other := newInstance("ns-3", 3, types.SubscriptionStatusClosed, base.Add(72*time.Hour))

	olderCreatedAt := base
	middleCreatedAt := base.Add(24 * time.Hour)
	newerCreatedAt := base.Add(48 * time.Hour)

	// The plan code lives on the plan table while instances only reference a plan
	// by id, so the plans those instances point at are inserted under the ids
	// they already use to give the plan_code filter something to resolve.
	insertTokenPlan(ctx, t, db, 1, "basic", "Basic", 1000)
	insertTokenPlan(ctx, t, db, 2, "pro", "Pro", 5000)
	insertTokenPlan(ctx, t, db, 3, "legacy", "Legacy", 100)

	t.Run("No filter lists every instance newest first", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{Per: 20, Page: 1})
		require.NoError(t, err)
		require.Equal(t, 4, totalsValue.Total)
		require.Equal(t, []int64{other.ID, newer.ID, middle.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Filters by several namespaces", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-1", "ns-2"}, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 3, totalsValue.Total)
		require.Equal(t, []int64{newer.ID, middle.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Filters by status", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			Status: ptr(types.SubscriptionStatusActive), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Len(t, instances, 1)
		require.Equal(t, older.ID, instances[0].ID)
	})

	t.Run("Filters by plan id", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanID: ptr(int64(1)), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, totalsValue.Total)
		require.Equal(t, []int64{newer.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Filters by plan code", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("basic"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, totalsValue.Total)
		require.Equal(t, []int64{newer.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Plan code matches a substring of the code", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("asi"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, totalsValue.Total)
		require.Equal(t, []int64{newer.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Plan code ignores case", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("BASIC"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, totalsValue.Total)
		require.Equal(t, []int64{newer.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Plan code wildcard characters stay literal", func(t *testing.T) {
		// A bare "%" would match every plan if it were not escaped, so it must
		// find nothing instead.
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("%"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, totalsValue.Total)
		require.Empty(t, instances)

		instances, totalsValue, err = store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("_"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, totalsValue.Total)
		require.Empty(t, instances)
	})

	t.Run("Plan code matches several plans sharing a fragment", func(t *testing.T) {
		// "e" is a substring of both "basic" and "legacy", so the filter has to
		// resolve to the plans of both and return every instance of either.
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("egac"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Equal(t, []int64{other.ID}, instanceIDs(instances))
	})

	t.Run("Plan code filters on a single plan", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("pro"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Equal(t, []int64{middle.ID}, instanceIDs(instances))
	})

	t.Run("Unknown plan code matches nothing", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			PlanCode: ptr("missing"), Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, totalsValue.Total)
		require.Empty(t, instances)
	})

	t.Run("Plan code combines with other filters", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs:  &[]string{"ns-1"},
			PlanCode: ptr("basic"),
			Per:      20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Equal(t, []int64{older.ID}, instanceIDs(instances))
	})

	t.Run("Filters by creation date range", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			CreatedAtStart: ptr(middleCreatedAt.Unix()),
			CreatedAtEnd:   ptr(newerCreatedAt.Unix()),
			Per:            20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Equal(t, middle.ID, instances[0].ID)
	})

	t.Run("Creation start is inclusive and end is exclusive", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			CreatedAtStart: ptr(olderCreatedAt.Unix()),
			CreatedAtEnd:   ptr(newerCreatedAt.Unix()),
			Per:            20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 2, totalsValue.Total)
		require.Equal(t, []int64{middle.ID, older.ID}, instanceIDs(instances))
	})

	t.Run("Combines every filter", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs:        &[]string{"ns-1"},
			Status:         ptr(types.SubscriptionStatusExpired),
			PlanID:         ptr(int64(2)),
			CreatedAtStart: ptr(middleCreatedAt.Unix()),
			Per:            20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 1, totalsValue.Total)
		require.Equal(t, middle.ID, instances[0].ID)
	})

	t.Run("Paginates while total covers every match", func(t *testing.T) {
		firstPage, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{Per: 2, Page: 1})
		require.NoError(t, err)
		require.Equal(t, 4, totalsValue.Total)
		require.Equal(t, []int64{other.ID, newer.ID}, instanceIDs(firstPage))

		secondPage, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{Per: 2, Page: 2})
		require.NoError(t, err)
		require.Equal(t, 4, totalsValue.Total)
		require.Equal(t, []int64{middle.ID, older.ID}, instanceIDs(secondPage))
	})

	t.Run("Fills namespace path and latest plan version", func(t *testing.T) {
		_, err := db.Core.NewInsert().Model(&database.Namespace{
			Path: "ns-one", UUID: "ns-1", UserID: 1, NamespaceType: database.UserNamespace,
		}).Exec(ctx)
		require.NoError(t, err)

		version := database.AccountTokenPlanVersion{
			AccountTokenPlanCommon: database.AccountTokenPlanCommon{
				PlanCode: "basic", PlanName: "Basic", PriceAmount: 1000,
				Currency: types.DEFAULT_CURRENCY, BillingPeriod: 1, PeriodUnit: types.UnitMonth,
			},
			PlanID: middle.PlanID, Version: 2,
		}
		_, err = db.Core.NewInsert().Model(&version).Exec(ctx, &version)
		require.NoError(t, err)
		require.NotZero(t, version.ID)
		versionID := version.ID

		first := insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: middle.ID, PlanVersionID: 1, PeriodNo: 1,
			StartAt: base.Unix(), EndAt: base.Add(24 * time.Hour).Unix(),
		})
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: middle.ID, PlanVersionID: versionID, PeriodNo: 2,
			StartAt: base.Add(24 * time.Hour).Unix(), EndAt: base.Add(48 * time.Hour).Unix(),
		})
		require.NotZero(t, first.ID)

		instances, _, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-1"}, Per: 20, Page: 1,
		})
		require.NoError(t, err)

		var found *database.AccountTokenPlanInstance
		for i := range instances {
			if instances[i].ID == middle.ID {
				found = &instances[i]
			}
		}
		require.NotNil(t, found)
		require.Equal(t, "ns-one", found.NamespacePath)
		require.NotNil(t, found.LatestPlanVersion)
		require.Equal(t, versionID, found.LatestPlanVersion.ID)
		require.Equal(t, int64(2), found.LatestPlanVersion.Version)
		require.Equal(t, "Basic", found.LatestPlanVersion.PlanName)
	})

	t.Run("Keeps display fields nil when nothing resolves", func(t *testing.T) {
		instances, _, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-unknown"}, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Empty(t, instances)
	})

	t.Run("Explicitly empty namespace list matches nothing", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{}, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, totalsValue.Total)
		require.Empty(t, instances)
	})

	t.Run("Sums period amount and cost per instance and across the filter", func(t *testing.T) {
		newPeriod := func(instanceID int64, periodNo int, amount, cost float64) {
			insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
				InstanceID:    instanceID,
				PlanVersionID: 1,
				PeriodNo:      int64(periodNo),
				StartAt:       base.Add(time.Duration(periodNo) * 24 * time.Hour).Unix(),
				EndAt:         base.Add(time.Duration(periodNo+1) * 24 * time.Hour).Unix(),
				Amount:        int64(amount),
				CostAmount:    cost,
			})
		}
		// older has 1000 + 2000 cents paid for 300 + 700 cents of cost.
		newPeriod(older.ID, 11, 1000, 300)
		newPeriod(older.ID, 12, 2000, 700)
		// middle has a single 500 cent period costing 125 cents.
		newPeriod(middle.ID, 11, 500, 125)

		instances, totals, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-1"}, Per: 20, Page: 1,
		})
		require.NoError(t, err)

		byID := map[int64]database.AccountTokenPlanInstance{}
		for i := range instances {
			byID[instances[i].ID] = instances[i]
		}
		require.Len(t, byID, 2)
		require.InDelta(t, 3000.0, byID[older.ID].TotalAmount, 1e-8)
		require.InDelta(t, 1000.0, byID[older.ID].TotalCost, 1e-8)
		require.InDelta(t, 500.0, byID[middle.ID].TotalAmount, 1e-8)
		require.InDelta(t, 125.0, byID[middle.ID].TotalCost, 1e-8)

		// Total counts the two matching instances, not their three periods.
		require.Equal(t, 2, totals.Total)
		require.InDelta(t, 3500.0, totals.TotalAmount, 1e-8)
		require.InDelta(t, 1125.0, totals.TotalCost, 1e-8)
	})

	t.Run("Totals cover every match beyond the returned page", func(t *testing.T) {
		// Four dedicated instances under their own namespace keep this test
		// independent of the periods other subtests add to the shared fixtures.
		pageIDs := make([]int64, 0, 4)
		for i := 0; i < 4; i++ {
			nsUUID := fmt.Sprintf("ns-page-%d", i)
			inst := newInstance(nsUUID, 1, types.SubscriptionStatusActive, base.Add(time.Duration(i)*24*time.Hour))
			pageIDs = append(pageIDs, inst.ID)
			// Plan version 0 keeps the period unique against periods other
			// subtests attach to the same shared fixtures.
			insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
				InstanceID:    inst.ID,
				PlanVersionID: 0,
				PeriodNo:      1,
				StartAt:       base.Unix(),
				EndAt:         base.Add(24 * time.Hour).Unix(),
				Amount:        100,
				CostAmount:    25,
			})
		}

		req := types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-page-0", "ns-page-1", "ns-page-2", "ns-page-3"},
			Per:     2, Page: 1,
		}
		firstPage, totals, err := store.ListInstances(ctx, req)
		require.NoError(t, err)
		require.Len(t, firstPage, 2)
		require.Equal(t, 4, totals.Total)
		// All four instances contribute 100 cents each, even though only two are
		// on the returned page.
		require.InDelta(t, 400.0, totals.TotalAmount, 1e-8)
		require.InDelta(t, 100.0, totals.TotalCost, 1e-8)

		req.Page = 2
		secondPage, totals, err := store.ListInstances(ctx, req)
		require.NoError(t, err)
		require.Len(t, secondPage, 2)
		require.Equal(t, 4, totals.Total)
		// The sums stay the same on the second page.
		require.InDelta(t, 400.0, totals.TotalAmount, 1e-8)
		require.InDelta(t, 100.0, totals.TotalCost, 1e-8)
		require.ElementsMatch(t, pageIDs, []int64{firstPage[0].ID, firstPage[1].ID, secondPage[0].ID, secondPage[1].ID})
	})

	t.Run("Unknown filter matches nothing", func(t *testing.T) {
		instances, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
			NsUUIDs: &[]string{"ns-missing"}, Per: 20, Page: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 0, totalsValue.Total)
		require.Empty(t, instances)
	})
}

func TestAccountTokenPlanStore_ListInstances_ExclusiveEndBoundary(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	location := config.GetGlobalTimeZone()
	lastOctober := time.Date(2026, 10, 31, 23, 59, 59, 0, location)
	firstNovember := time.Date(2026, 11, 1, 0, 0, 0, 0, location)

	insertTokenPlan(ctx, t, db, 1, "boundary", "Boundary", 1000)
	newInstance := func(createdAt time.Time) database.AccountTokenPlanInstance {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID:    "ns-boundary",
			PlanID:    1,
			Status:    types.SubscriptionStatusActive,
			StartAt:   createdAt.Unix(),
			EndAt:     createdAt.Add(24 * time.Hour).Unix(),
			CreatedBy: "user-1",
		})
		_, err := db.Core.NewUpdate().
			Model((*database.AccountTokenPlanInstance)(nil)).
			Set("created_at = ?", createdAt).
			Where("id = ?", instance.ID).
			Exec(ctx)
		require.NoError(t, err)
		return instance
	}

	lastOctoberInstance := newInstance(lastOctober)
	newInstance(firstNovember)

	instances, totals, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
		CreatedAtStart: ptr(time.Date(2026, 10, 31, 0, 0, 0, 0, location).Unix()),
		CreatedAtEnd:   ptr(firstNovember.Unix()),
		Per:            20,
		Page:           1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, totals.Total)
	require.Equal(t, []int64{lastOctoberInstance.ID}, instanceIDs(instances))
}

// TestAccountTokenPlanStore_ListInstancesByPlanCodeTotals checks that a plan code
// filter narrows the summed amount and cost to the instances of that plan, not
// just the count and the page. It builds its own plans, instances and periods so
// the sums stay independent of the shared fixtures of the listing test.
func TestAccountTokenPlanStore_ListInstancesByPlanCodeTotals(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	base := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	basicPlan := insertTokenPlan(ctx, t, db, autoIncrementPlanID, "totals-basic", "Totals Basic", 1000)
	proPlan := insertTokenPlan(ctx, t, db, autoIncrementPlanID, "totals-pro", "Totals Pro", 5000)

	newInstanceWithPeriod := func(nsUUID string, planID, amount, cost int64) database.AccountTokenPlanInstance {
		instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
			NsUUID: nsUUID, PlanID: planID, Status: types.SubscriptionStatusActive,
			StartAt: base.Unix(), EndAt: base.Add(24 * time.Hour).Unix(), CreatedBy: "user-1",
		})
		insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
			InstanceID: instance.ID, PlanVersionID: 0, PeriodNo: 1,
			StartAt: base.Unix(), EndAt: base.Add(24 * time.Hour).Unix(),
			Amount: amount, CostAmount: float64(cost),
		})
		return instance
	}

	// Two instances of the basic plan and one of the pro plan, so a filter on
	// either code has to sum a different subset.
	newInstanceWithPeriod("ns-totals-1", basicPlan.ID, 100, 10)
	newInstanceWithPeriod("ns-totals-2", basicPlan.ID, 200, 20)
	newInstanceWithPeriod("ns-totals-3", proPlan.ID, 400, 40)

	_, totalsValue, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
		PlanCode: ptr("totals-basic"), Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, totalsValue.Total)
	require.InDelta(t, 300.0, totalsValue.TotalAmount, 1e-8)
	require.InDelta(t, 30.0, totalsValue.TotalCost, 1e-8)

	_, totalsValue, err = store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
		PlanCode: ptr("totals-pro"), Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, totalsValue.Total)
	require.InDelta(t, 400.0, totalsValue.TotalAmount, 1e-8)
	require.InDelta(t, 40.0, totalsValue.TotalCost, 1e-8)

	_, totalsValue, err = store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{Per: 20, Page: 1})
	require.NoError(t, err)
	require.Equal(t, 3, totalsValue.Total)
	require.InDelta(t, 700.0, totalsValue.TotalAmount, 1e-8)
	require.InDelta(t, 70.0, totalsValue.TotalCost, 1e-8)
}

func TestAccountTokenPlanStore_ListInstancesByPlanCodeIncludesDeletedPlan(t *testing.T) {
	ctx := context.Background()
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewAccountTokenPlanStoreWithDB(db)

	base := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	plan := insertTokenPlan(ctx, t, db, autoIncrementPlanID, "review-basic", "Review Basic", 1000)
	instance := insertTokenPlanInstance(ctx, t, db, database.AccountTokenPlanInstance{
		NsUUID:    "ns-deleted-plan",
		PlanID:    plan.ID,
		Status:    types.SubscriptionStatusClosed,
		StartAt:   base.Unix(),
		EndAt:     base.Add(24 * time.Hour).Unix(),
		CreatedBy: "user-1",
	})
	insertTokenPlanPeriod(ctx, t, db, database.AccountTokenPlanInstancePeriod{
		InstanceID: instance.ID,
		PeriodNo:   1,
		StartAt:    base.Unix(),
		EndAt:      base.Add(24 * time.Hour).Unix(),
		Amount:     100,
		CostAmount: 10,
	})

	require.NoError(t, store.Delete(ctx, plan.ID))
	_, err := store.GetByID(ctx, plan.ID)
	require.Error(t, err, "the plan must be soft deleted for this regression test")

	byID, byIDTotals, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
		PlanID: ptr(plan.ID),
		Status: ptr(types.SubscriptionStatusClosed),
		Per:    20,
		Page:   1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, byIDTotals.Total)
	require.Equal(t, []int64{instance.ID}, instanceIDs(byID))
	require.InDelta(t, 100.0, byIDTotals.TotalAmount, 1e-8)
	require.InDelta(t, 10.0, byIDTotals.TotalCost, 1e-8)

	byCode, byCodeTotals, err := store.ListInstances(ctx, types.AcctTokenPlanInstanceListReq{
		PlanCode: ptr("review-basic"),
		Status:   ptr(types.SubscriptionStatusClosed),
		Per:      20,
		Page:     1,
	})
	require.NoError(t, err)
	require.Equal(t, byIDTotals, byCodeTotals)
	require.Equal(t, instanceIDs(byID), instanceIDs(byCode))
}

func instanceIDs(instances []database.AccountTokenPlanInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for i := range instances {
		ids = append(ids, instances[i].ID)
	}
	return ids
}

// ptr returns a pointer to v, used to build the optional filters of the
// token plan instance listing request.
func ptr[T any](v T) *T {
	return &v
}
