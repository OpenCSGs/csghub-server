package database_test

import (
	"context"
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

func insertTokenPlanInstance(ctx context.Context, t *testing.T, db *database.DB,
	instance database.AccountTokenPlanInstance) database.AccountTokenPlanInstance {
	t.Helper()

	_, err := db.Core.NewInsert().Model(&instance).Exec(ctx, &instance)
	require.NoError(t, err)
	return instance
}

func insertTokenPlanPeriod(ctx context.Context, t *testing.T, db *database.DB,
	period database.AccountTokenPlanInstancePeriod) database.AccountTokenPlanInstancePeriod {
	t.Helper()

	_, err := db.Core.NewInsert().Model(&period).Exec(ctx, &period)
	require.NoError(t, err)
	return period
}
