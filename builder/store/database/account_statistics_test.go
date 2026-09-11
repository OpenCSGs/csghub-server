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

func TestAccountStatisticsStore_SummaryByUserIDAndDate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	stats := []database.AccountStatistics{
		{
			UserUUID: "user1", Scene: types.SceneModelInference, CustomerID: "c1",
			EventDate: dt, Consumption: 10, PromptToken: 100, PromptCachedToken: 20,
			CompletionToken: 50, Count: 5, Duration: 30,
		},
		{
			UserUUID: "user1", Scene: types.SceneModelInference, CustomerID: "c2",
			EventDate: dt.Add(1 * 24 * time.Hour), Consumption: 20, PromptToken: 200,
			PromptCachedToken: 40, CompletionToken: 100, Count: 10, Duration: 60,
		},
		{
			UserUUID: "user1", Scene: types.SceneSpace, CustomerID: "c3",
			EventDate: dt, Consumption: 5, PromptToken: 0, PromptCachedToken: 0,
			CompletionToken: 0, Count: 2, Duration: 15,
		},
		{
			UserUUID: "user2", Scene: types.SceneModelInference, CustomerID: "c4",
			EventDate: dt, Consumption: 99, PromptToken: 999, PromptCachedToken: 999,
			CompletionToken: 999, Count: 999, Duration: 999,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&stats).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountStatisticsStoreWithDB(db)

	t.Run("summary grouped by scene without scene filter", func(t *testing.T) {
		res, err := store.SummaryByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "user1",
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))

		sceneMap := map[types.SceneType]database.SceneSummary{}
		for _, s := range res.Data {
			sceneMap[s.Scene] = s
		}

		inferenceSummary := sceneMap[types.SceneModelInference]
		require.Equal(t, float64(30), inferenceSummary.TotalConsumption)
		require.Equal(t, float64(300), inferenceSummary.TotalPromptToken)
		require.Equal(t, float64(60), inferenceSummary.TotalPromptCachedToken)
		require.Equal(t, float64(150), inferenceSummary.TotalCompletionToken)
		require.Equal(t, float64(15), inferenceSummary.TotalCount)
		require.Equal(t, float64(90), inferenceSummary.TotalDuration)

		spaceSummary := sceneMap[types.SceneSpace]
		require.Equal(t, float64(5), spaceSummary.TotalConsumption)
		require.Equal(t, float64(0), spaceSummary.TotalPromptToken)
		require.Equal(t, float64(2), spaceSummary.TotalCount)
		require.Equal(t, float64(15), spaceSummary.TotalDuration)
	})

	t.Run("summary with scene filter", func(t *testing.T) {
		res, err := store.SummaryByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "user1",
			Scene:      types.SceneModelInference,
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(res.Data))

		require.Equal(t, types.SceneModelInference, res.Data[0].Scene)
		require.Equal(t, float64(30), res.Data[0].TotalConsumption)
		require.Equal(t, float64(300), res.Data[0].TotalPromptToken)
		require.Equal(t, float64(60), res.Data[0].TotalPromptCachedToken)
		require.Equal(t, float64(150), res.Data[0].TotalCompletionToken)
		require.Equal(t, float64(15), res.Data[0].TotalCount)
		require.Equal(t, float64(90), res.Data[0].TotalDuration)
	})

	t.Run("summary with date range excluding records", func(t *testing.T) {
		res, err := store.SummaryByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "user1",
			Scene:      types.SceneModelInference,
			StartDate:  dt.Add(-3 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
		})
		require.Nil(t, err)
		require.Equal(t, 0, len(res.Data))
	})

	t.Run("summary for different user", func(t *testing.T) {
		res, err := store.SummaryByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "user2",
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(res.Data))
		require.Equal(t, types.SceneModelInference, res.Data[0].Scene)
		require.Equal(t, float64(99), res.Data[0].TotalConsumption)
	})
}

func TestAccountStatisticsStore_ListByUserIDAndDate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	stats := []database.AccountStatistics{
		{
			UserUUID: "foo", Scene: types.SceneModelInference, CustomerID: "c1",
			EventDate: dt, Consumption: 10, PromptToken: 100, CompletionToken: 50,
		},
		{
			UserUUID: "foo", Scene: types.SceneModelInference, CustomerID: "c2",
			EventDate: dt.Add(1 * 24 * time.Hour), Consumption: 20, PromptToken: 200, CompletionToken: 100,
		},
		{
			UserUUID: "foo", Scene: types.SceneSpace, CustomerID: "c3",
			EventDate: dt, Consumption: 5,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&stats).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountStatisticsStoreWithDB(db)

	t.Run("list statistics for user and scene", func(t *testing.T) {
		res, err := store.ListByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "foo",
			Scene:      types.SceneModelInference,
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))
		require.Equal(t, 2, res.Total)
		require.Equal(t, float64(30), res.TotalConsumption)
		require.Equal(t, float64(300), res.TotalPromptToken)
		require.Equal(t, float64(150), res.TotalCompletionToken)
	})

	t.Run("list statistics with wrong scene returns empty", func(t *testing.T) {
		res, err := store.ListByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: "foo",
			Scene:      types.SceneEvaluation,
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 0, len(res.Data))
	})

	t.Run("pagination totals are independent of page", func(t *testing.T) {
		baseReq := types.AcctBillsReq{
			TargetUUID: "foo",
			Scene:      types.SceneModelInference,
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			Per:        1,
		}
		page1Req, page2Req := baseReq, baseReq
		page1Req.Page, page2Req.Page = 1, 2

		page1, err := store.ListByUserIDAndDate(ctx, page1Req)
		require.Nil(t, err)
		page2, err := store.ListByUserIDAndDate(ctx, page2Req)
		require.Nil(t, err)

		// Per=1 over 2 grouped rows: each page returns a different instance.
		require.Equal(t, 1, len(page1.Data))
		require.Equal(t, 1, len(page2.Data))
		require.NotEqual(t, page1.Data[0].InstanceName, page2.Data[0].InstanceName)

		// Regression guard: totals must be identical across pages. If a
		// LIMIT/OFFSET leaked into the grouped_items CTE, the aggregates
		// would shrink to the current page (10 vs 20 instead of 30).
		require.Equal(t, page1.Total, page2.Total)
		require.Equal(t, page1.TotalConsumption, page2.TotalConsumption)
		require.Equal(t, page1.TotalPromptToken, page2.TotalPromptToken)
		require.Equal(t, page1.TotalPromptCachedToken, page2.TotalPromptCachedToken)
		require.Equal(t, page1.TotalCompletionToken, page2.TotalCompletionToken)
		require.Equal(t, page1.TotalCount, page2.TotalCount)
		require.Equal(t, page1.TotalDuration, page2.TotalDuration)

		// And they equal the full-result aggregates (2 inference groups: 10 + 20).
		require.Equal(t, 2, page1.Total)
		require.Equal(t, float64(30), page1.TotalConsumption)
		require.Equal(t, float64(300), page1.TotalPromptToken)
		require.Equal(t, float64(150), page1.TotalCompletionToken)
	})
}

func TestAccountStatisticsStore_ListStatisticsDetailByUserID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	stats := []database.AccountStatistics{
		{
			UserUUID: "foo", Scene: types.SceneModelInference, CustomerID: "c1",
			EventDate: dt, Consumption: 10, PromptToken: 100, CompletionToken: 50,
		},
		{
			UserUUID: "foo", Scene: types.SceneModelInference, CustomerID: "c2",
			EventDate: dt.Add(1 * 24 * time.Hour), Consumption: 20, PromptToken: 200, CompletionToken: 100,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&stats).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountStatisticsStoreWithDB(db)

	t.Run("list detail for user and scene", func(t *testing.T) {
		res, err := store.ListStatisticsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.SceneModelInference),
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))
		require.Equal(t, 2, res.Total)
	})

	t.Run("list detail with instance name filter", func(t *testing.T) {
		res, err := store.ListStatisticsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID:   "foo",
			Scene:        int(types.SceneModelInference),
			StartDate:    dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:      dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			InstanceName: "c1",
			Per:          20,
			Page:         1,
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(res.Data))
		require.Equal(t, 1, res.Total)
		require.Equal(t, "c1", res.Data[0].CustomerID)
	})

	t.Run("list detail with pagination", func(t *testing.T) {
		res, err := store.ListStatisticsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.SceneModelInference),
			StartDate:  dt.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(3 * 24 * time.Hour).Format(time.RFC3339),
			Per:        1,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(res.Data))
		require.Equal(t, 2, res.Total)
	})
}
