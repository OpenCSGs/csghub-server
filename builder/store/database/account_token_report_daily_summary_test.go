package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountTokenReportDailySummaryStore_UpsertAndReport(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountTokenReportDailySummaryStoreWithDB(db)

	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	seed := []database.AccountStatement{
		// two rows of the same report group for user-a: same ns/token/model/provider,
		// different upstream (storage grain finer than report grain)
		{EventUUID: uuid.New(), UserUUID: "user-a", TokenID: 11, ResourceID: "thirdparty://deepseek-r1", Provider: "硅基流动", UpstreamID: 1, Scene: types.SceneModelServerless, Value: -42050, CostAmount: 28500, PromptToken: 30000000, PromptCachedToken: 10000000, CompletionToken: 12000000, CreatedAt: day.Add(1 * time.Hour)},
		{EventUUID: uuid.New(), UserUUID: "user-a", TokenID: 11, ResourceID: "thirdparty://deepseek-r1", Provider: "硅基流动", UpstreamID: 2, Scene: types.SceneModelServerless, Value: -10000, CostAmount: 5000, PromptToken: 15200000, PromptCachedToken: 2000000, CompletionToken: 6500000, CreatedAt: day.Add(2 * time.Hour)},
		// another group: different model/provider
		{EventUUID: uuid.New(), UserUUID: "user-a", TokenID: 11, ResourceID: "thirdparty://qwen2.5-72b", Provider: "阿里云百炼", UpstreamID: 3, Scene: types.SceneModelServerless, Value: -9800, CostAmount: 7200, PromptToken: 12000000, CompletionToken: 6000000, CreatedAt: day.Add(3 * time.Hour)},
		// multimodal scene row
		{EventUUID: uuid.New(), UserUUID: "user-b", TokenID: 12, ResourceID: "thirdparty://qwen-image", Provider: "阿里云百炼", UpstreamID: 4, Scene: types.SceneMultiModalServerless, Value: -150, CostAmount: 100, DataType: "image", Resolution: "1024", Duration: 0, CreatedAt: day.Add(4 * time.Hour)},
		// excluded: non-token scene
		{EventUUID: uuid.New(), UserUUID: "user-a", Scene: types.SceneSpace, Value: -7, CreatedAt: day.Add(5 * time.Hour)},
		// excluded: next day
		{EventUUID: uuid.New(), UserUUID: "user-a", TokenID: 11, ResourceID: "thirdparty://deepseek-r1", Provider: "硅基流动", Scene: types.SceneModelServerless, Value: -999, CreatedAt: day.AddDate(0, 0, 1)},
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	rows, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	require.Equal(t, int64(4), rows) // 2 user-a deepseek upstreams + qwen + multimodal

	// re-run is idempotent (overwrite, not double count)
	rows, err = store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	require.Equal(t, int64(4), rows)

	var summaries []database.AccountTokenReportDailySummary
	err = db.Core.NewSelect().Model((*database.AccountTokenReportDailySummary)(nil)).Scan(ctx, &summaries)
	require.Nil(t, err)
	require.Len(t, summaries, 4)

	// report grain collapses upstream_id: 3 groups
	report, total, err := store.ListReport(ctx, types.TokenReportReq{
		StartTime: "2026-09-01",
		EndTime:   "2026-09-02",
	})
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, report, 3)

	byKey := map[string]database.TokenReportRow{}
	for _, r := range report {
		byKey[r.NsUUID+"/"+r.ResourceID] = r
	}
	ds := byKey["user-a/thirdparty://deepseek-r1"]
	require.Equal(t, int64(2), ds.CallCount)
	require.InDelta(t, 45200000, ds.PromptToken, 1e-6)
	require.InDelta(t, 12000000, ds.PromptCachedToken, 1e-6)
	require.InDelta(t, 18500000, ds.CompletionToken, 1e-6)
	require.InDelta(t, -52050, ds.RevenueAmount, 1e-6)
	require.InDelta(t, 33500, ds.CostAmount, 1e-6)

	totals, err := store.SumReport(ctx, types.TokenReportReq{
		StartTime: "2026-09-01",
		EndTime:   "2026-09-02",
	})
	require.Nil(t, err)
	require.Equal(t, int64(4), totals.CallCount)
	require.InDelta(t, -52050-9800-150, totals.RevenueAmount, 1e-6)
	require.InDelta(t, 33500+7200+100, totals.CostAmount, 1e-6)

	// filter by provider
	filtered, total, err := store.ListReport(ctx, types.TokenReportReq{
		StartTime: "2026-09-01",
		EndTime:   "2026-09-02",
		Provider:  "阿里云百炼",
	})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, filtered, 2)
}

func TestAccountTokenReportDailySummaryStore_CheckpointTx(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountTokenReportDailySummaryStoreWithDB(db)
	cpStore := database.NewCronCheckpointStoreWithDB(db)

	day := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	_, err := store.UpsertSummaryAndCheckpoint(ctx, day, "token_report_daily_summary")
	require.Nil(t, err)

	lastDate, err := cpStore.GetLastDate(ctx, "token_report_daily_summary")
	require.Nil(t, err)
	require.Equal(t, day, lastDate)

	// checkpoint advances on re-run
	next := day.AddDate(0, 0, 1)
	_, err = store.UpsertSummaryAndCheckpoint(ctx, next, "token_report_daily_summary")
	require.Nil(t, err)
	lastDate, err = cpStore.GetLastDate(ctx, "token_report_daily_summary")
	require.Nil(t, err)
	require.Equal(t, next, lastDate)
}

func TestAccountTokenReportDailySummaryStore_TimezoneBoundary(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountTokenReportDailySummaryStoreWithDB(db)

	// The activity passes calendar dates in the global timezone (Asia/Shanghai).
	// created_at is timestamptz: a 23:30 +08 row belongs to stat_date D even
	// though it is 15:30 UTC, and a 01:00 +08 row on D+1 must not leak into D.
	tz := time.FixedZone("Asia/Shanghai", 8*3600)
	day := time.Date(2026, 9, 3, 0, 0, 0, 0, tz)

	seed := []database.AccountStatement{
		{EventUUID: uuid.New(), UserUUID: "user-a", ResourceID: "thirdparty://m", Provider: "p", Scene: types.SceneModelServerless, Value: -100, CostAmount: 50, PromptToken: 10, CreatedAt: day.Add(23*time.Hour + 30*time.Minute)},    // D 23:30 +08
		{EventUUID: uuid.New(), UserUUID: "user-a", ResourceID: "thirdparty://m", Provider: "p", Scene: types.SceneModelServerless, Value: -200, CostAmount: 60, PromptToken: 20, CreatedAt: day.Add(1 * time.Hour)},                    // D 01:00 +08
		{EventUUID: uuid.New(), UserUUID: "user-a", ResourceID: "thirdparty://m", Provider: "p", Scene: types.SceneModelServerless, Value: -999, CostAmount: 999, PromptToken: 999, CreatedAt: day.AddDate(0, 0, 1).Add(1 * time.Hour)}, // D+1 01:00 +08
	}
	for _, s := range seed {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	rows, err := store.UpsertSummary(ctx, day)
	require.Nil(t, err)
	require.Equal(t, int64(1), rows)

	report, total, err := store.ListReport(ctx, types.TokenReportReq{StartTime: "2026-09-03", EndTime: "2026-09-04"})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "2026-09-03", report[0].StatDate.Format("2006-01-02"))
	require.Equal(t, int64(2), report[0].CallCount)
	require.InDelta(t, 30, report[0].PromptToken, 1e-6)
	require.InDelta(t, -300, report[0].RevenueAmount, 1e-6)
	require.InDelta(t, 110, report[0].CostAmount, 1e-6)
}
