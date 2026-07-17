package database_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountStatementDailySummaryStore_UpsertSummary(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	day := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	// seed one day of raw statements for two users / skus, plus a next-day row
	seed := []database.AccountStatement{
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 10, Consumption: 1, CreatedAt: day.Add(1 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 20, Consumption: 2, CreatedAt: day.Add(2 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 2, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 5, Consumption: 0.5, CreatedAt: day.Add(3 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-b", SkuID: 1, Scene: types.SceneSpace, CustomerID: "inst-2", Value: 7, Consumption: 3, CreatedAt: day.Add(4 * time.Hour)},
		// next day row, must be excluded from the day summary
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 100, Consumption: 100, CreatedAt: day.AddDate(0, 0, 1).Add(1 * time.Hour)},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	rows, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	// 3 groups for `day`: (user-a,sku1,inf,inst1),(user-a,sku2,inf,inst1),(user-b,sku1,space,inst2)
	require.Equal(t, int64(3), rows)

	// verify the aggregated values
	var summaries []database.AccountStatementDailySummary
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).Scan(ctx, &summaries)
	require.Nil(t, err)
	require.Len(t, summaries, 3)

	byKey := map[string]database.AccountStatementDailySummary{}
	for _, s := range summaries {
		byKey[s.UserUUID+"|"+strconv.FormatInt(s.SkuID, 10)+"|"+strconv.Itoa(int(s.Scene))+"|"+s.CustomerID] = s
	}

	a1 := byKey["user-a|1|" + strconv.Itoa(int(types.SceneModelInference)) + "|inst-1"]
	require.Equal(t, 30.0, a1.TotalValue)
	require.Equal(t, 3.0, a1.TotalConsumption)
	require.Equal(t, int64(2), a1.TotalCount)

	a2 := byKey["user-a|2|" + strconv.Itoa(int(types.SceneModelInference)) + "|inst-1"]
	require.Equal(t, 5.0, a2.TotalValue)
	require.Equal(t, int64(1), a2.TotalCount)

	b1 := byKey["user-b|1|" + strconv.Itoa(int(types.SceneSpace)) + "|inst-2"]
	require.Equal(t, 7.0, b1.TotalValue)

	// re-run must be idempotent (upsert overwrites, no duplicate rows)
	rows2, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	require.Equal(t, int64(3), rows2)

	var count int
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).
		ColumnExpr("COUNT(*)").Scan(ctx, &count)
	require.Nil(t, err)
	require.Equal(t, 3, count)
}

func TestAccountStatementDailySummaryStore_UpsertSummary_NullDimensionsIdempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	day := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	// historical data with REAL NULL sku_id and NULL customer_id (pre-sku-era rows).
	// A plain unique index treats NULL != NULL, so without COALESCE normalization
	// the ON CONFLICT would not match and re-runs would insert duplicate rows.
	for _, v := range []float64{-10, -20} {
		_, err := db.Core.ExecContext(ctx, `INSERT INTO account_statements
		  (event_uuid, user_uuid, sku_id, scene, customer_id, value, consumption, created_at)
		  VALUES (?, ?, NULL, ?, NULL, ?, ?, ?)`,
			uuid.New(), "user-a", int(types.SceneModelInference), v, 1.0, day.Add(time.Hour))
		require.Nil(t, err)
	}

	// first run
	_, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)

	// re-run must be idempotent: COALESCE normalizes NULL dims to 0/'', so the
	// unique index matches and the row is overwritten (not duplicated).
	_, err = store.UpsertSummary(ctx, day)
	require.Nil(t, err)

	var count int
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).
		ColumnExpr("COUNT(*)").Scan(ctx, &count)
	require.Nil(t, err)
	require.Equal(t, 1, count, "re-run must not insert duplicate rows for NULL-dimension groups")

	var sum database.AccountStatementDailySummary
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).Scan(ctx, &sum)
	require.Nil(t, err)
	require.Equal(t, -30.0, sum.TotalValue, "re-run must not double the total")
	require.Equal(t, int64(0), sum.SkuID, "NULL sku_id normalized to 0")
	require.Equal(t, "", sum.CustomerID, "NULL customer_id normalized to empty string")
}

func TestAccountStatementDailySummaryStore_UpsertSummary_MixedNullAndZeroDimensions(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	day := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	// old data mixes real NULL, 0, and '' for the same logical group.
	// GROUP BY on raw columns splits them into multiple groups, but COALESCE
	// in SELECT maps them to the same normalized key — so the INSERT would try
	// to upsert the same unique row twice in one statement (SQLSTATE 21000)
	// unless GROUP BY also normalizes with COALESCE.
	rows := []struct {
		skuID      any
		customerID any
		value      float64
	}{
		{nil, nil, -10},    // NULL, NULL
		{0, "", -20},       // zero, empty
		{nil, "", -30},     // NULL, empty
		{0, nil, -40},      // zero, NULL
	}
	for _, r := range rows {
		_, err := db.Core.ExecContext(ctx, `INSERT INTO account_statements
		  (event_uuid, user_uuid, sku_id, scene, customer_id, value, consumption, created_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New(), "user-a", r.skuID, int(types.SceneModelInference), r.customerID, r.value, 1.0, day.Add(time.Hour))
		require.Nil(t, err)
	}

	// must not error with "ON CONFLICT DO UPDATE command cannot affect row a second time"
	_, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)

	var count int
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).
		ColumnExpr("COUNT(*)").Scan(ctx, &count)
	require.Nil(t, err)
	require.Equal(t, 1, count, "all NULL/0/'' variants collapse into one normalized group")

	var sum database.AccountStatementDailySummary
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).Scan(ctx, &sum)
	require.Nil(t, err)
	require.Equal(t, -100.0, sum.TotalValue) // -10-20-30-40
}

func TestAccountStatementDailySummaryStore_UpsertSummary_ExcludesChargeScenes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	day := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	// seed: one consumption row (negative value) + one recharge row (positive value)
	// recharge scenes (ScenePortalCharge/SceneCashCharge) must be excluded from the rollup
	seed := []database.AccountStatement{
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: -10, Consumption: 1, CreatedAt: day.Add(1 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.ScenePortalCharge, CustomerID: "", Value: 100, Consumption: 0, CreatedAt: day.Add(2 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneCashCharge, CustomerID: "", Value: 50, Consumption: 0, CreatedAt: day.Add(3 * time.Hour)},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	rows, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	// only the consumption group lands; the two recharge rows are excluded
	require.Equal(t, int64(1), rows)

	var summaries []database.AccountStatementDailySummary
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).Scan(ctx, &summaries)
	require.Nil(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, types.SceneModelInference, summaries[0].Scene)
	require.Equal(t, -10.0, summaries[0].TotalValue) // recharge 100+50 not summed in
}

func TestAccountStatementDailySummaryStore_ListByUserAndSku(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	jul1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jul15 := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	aug1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// seed summary rows across two months for two users
	seed := []database.AccountStatementDailySummary{
		{BillDate: jul1, UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", TotalValue: 10, TotalConsumption: 1, TotalCount: 1, MinID: 100, MinCreatedAt: jul1},
		{BillDate: jul15, UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", TotalValue: 20, TotalConsumption: 2, TotalCount: 1, MinID: 200, MinCreatedAt: jul15},
		{BillDate: aug1, UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", TotalValue: 999, TotalConsumption: 999, TotalCount: 1, MinID: 300, MinCreatedAt: aug1},
		{BillDate: jul1, UserUUID: "user-b", SkuID: 1, Scene: types.SceneSpace, CustomerID: "inst-2", TotalValue: 5, TotalConsumption: 0.5, TotalCount: 1, MinID: 400, MinCreatedAt: jul1},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	// query July for all users (bill_date range maps month start..end+1day)
	results, total, err := store.ListByUserAndSku(ctx, types.ActStatementsReq{
		StartTime: "2026-07-01",
		EndTime:   "2026-08-01",
		Per:       10,
		Page:      1,
	})
	require.Nil(t, err)
	// groups in July: (user-a,sku1,inf,inst1) and (user-b,sku1,space,inst2)
	require.Equal(t, 2, total)
	require.Len(t, results, 2)

	// the user-a July group should sum the two July daily rows
	var aGroup *database.UserSkuStatement
	for i := range results {
		if results[i].UserUUID == "user-a" {
			aGroup = &results[i]
		}
	}
	require.NotNil(t, aGroup)
	require.Equal(t, 30.0, aGroup.TotalValue)
	require.Equal(t, 3.0, aGroup.TotalConsumption)
	require.Equal(t, int64(100), aGroup.ID) // MIN(min_id)

	// filter by user-a only
	results, total, err = store.ListByUserAndSku(ctx, types.ActStatementsReq{
		UserUUID:  "user-a",
		StartTime: "2026-07-01",
		EndTime:   "2026-08-01",
		Per:       10,
		Page:      1,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)
	require.Equal(t, "user-a", results[0].UserUUID)
	require.Equal(t, 30.0, results[0].TotalValue)
}

func TestAccountStatementDailySummaryStore_ListByUserAndSku_FiltersAndPaging(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementDailySummaryStoreWithDB(db)

	jul1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// 3 distinct groups in July for the same user, different scenes/instances
	seed := []database.AccountStatementDailySummary{
		{BillDate: jul1, UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", TotalValue: 10, TotalCount: 1, MinID: 10, MinCreatedAt: jul1},
		{BillDate: jul1, UserUUID: "user-a", SkuID: 1, Scene: types.SceneSpace, CustomerID: "inst-2", TotalValue: 20, TotalCount: 1, MinID: 20, MinCreatedAt: jul1},
		{BillDate: jul1, UserUUID: "user-a", SkuID: 2, Scene: types.SceneModelInference, CustomerID: "inst-1", TotalValue: 30, TotalCount: 1, MinID: 30, MinCreatedAt: jul1},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	// filter by scene -> only the SceneSpace group
	results, total, err := store.ListByUserAndSku(ctx, types.ActStatementsReq{
		Scene:     types.SceneSpace,
		StartTime: "2026-07-01",
		EndTime:   "2026-08-01",
		Per:       10,
		Page:      1,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)
	require.Equal(t, types.SceneSpace, results[0].Scene)

	// filter by instance -> only inst-2 groups
	results, total, err = store.ListByUserAndSku(ctx, types.ActStatementsReq{
		InstanceName: "inst-2",
		StartTime:    "2026-07-01",
		EndTime:      "2026-08-01",
		Per:          10,
		Page:         1,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)
	require.Equal(t, "inst-2", results[0].CustomerID)

	// paging: per=2 page=1 -> 2 rows, total still 3
	results, total, err = store.ListByUserAndSku(ctx, types.ActStatementsReq{
		StartTime: "2026-07-01",
		EndTime:   "2026-08-01",
		Per:       2,
		Page:      1,
	})
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, results, 2)

	// paging: per=2 page=2 -> 1 row
	results, total, err = store.ListByUserAndSku(ctx, types.ActStatementsReq{
		StartTime: "2026-07-01",
		EndTime:   "2026-08-01",
		Per:       2,
		Page:      2,
	})
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, results, 1)
}

func TestAccountStatementDailySummaryStore_UpsertSummaryAndCheckpoint(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	summaryStore := database.NewAccountStatementDailySummaryStoreWithDB(db)
	cpStore := database.NewCronCheckpointStoreWithDB(db)

	day := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	// seed one day of raw statements
	seed := []database.AccountStatement{
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 10, Consumption: 1, CreatedAt: day.Add(1 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", SkuID: 1, Scene: types.SceneModelInference, CustomerID: "inst-1", Value: 20, Consumption: 2, CreatedAt: day.Add(2 * time.Hour)},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	// checkpoint should be empty before first run
	last, err := cpStore.GetLastDate(ctx, "statement_daily_summary")
	require.Nil(t, err)
	require.True(t, last.IsZero())

	// run: data + checkpoint in one tx
	rows, err := summaryStore.UpsertSummaryAndCheckpoint(ctx, day, "statement_daily_summary")
	require.Nil(t, err)
	require.Equal(t, int64(1), rows)

	// checkpoint now recorded as the processed day
	last, err = cpStore.GetLastDate(ctx, "statement_daily_summary")
	require.Nil(t, err)
	require.Equal(t, day.Format("2006-01-02"), last.Format("2006-01-02"))

	// summary data was written
	var summaries []database.AccountStatementDailySummary
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).Scan(ctx, &summaries)
	require.Nil(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, 30.0, summaries[0].TotalValue)

	// re-run is idempotent: data overwrites same value, checkpoint unchanged
	rows2, err := summaryStore.UpsertSummaryAndCheckpoint(ctx, day, "statement_daily_summary")
	require.Nil(t, err)
	require.Equal(t, int64(1), rows2)

	var count int
	err = db.Core.NewSelect().Model((*database.AccountStatementDailySummary)(nil)).
		ColumnExpr("COUNT(*)").Scan(ctx, &count)
	require.Nil(t, err)
	require.Equal(t, 1, count)
}

func TestCronCheckpointStore_GetSave(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCronCheckpointStoreWithDB(db)

	// empty -> zero time
	last, err := store.GetLastDate(ctx, "some-job")
	require.Nil(t, err)
	require.True(t, last.IsZero())

	// save
	d := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	err = store.SaveLastDate(ctx, "some-job", d)
	require.Nil(t, err)
	last, err = store.GetLastDate(ctx, "some-job")
	require.Nil(t, err)
	require.Equal(t, d.Format("2006-01-02"), last.Format("2006-01-02"))

	// upsert (update existing)
	d2 := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	err = store.SaveLastDate(ctx, "some-job", d2)
	require.Nil(t, err)
	last, err = store.GetLastDate(ctx, "some-job")
	require.Nil(t, err)
	require.Equal(t, d2.Format("2006-01-02"), last.Format("2006-01-02"))

	var count int
	err = db.Core.NewSelect().Model((*database.CronCheckpoint)(nil)).
		ColumnExpr("COUNT(*)").Scan(ctx, &count)
	require.Nil(t, err)
	require.Equal(t, 1, count) // no duplicate rows
}
