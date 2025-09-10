package database_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	"testing"
	"time"
)

func TestStatSnapStore_AddAndGet(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ss := database.NewStatSnapStoreWithDB(db)

	snapshot := &database.StatSnapshot{
		TargetType:   types.StatTargetUser,
		DateType:     types.StatDateMonth,
		SnapshotDate: "2025-05-01",
		TrendData:    database.Trend{"1": 10, "2": 20},
		TotalCount:   30,
		NewCount:     5,
	}

	err := ss.Add(ctx, snapshot)
	require.NoError(t, err)

	req := types.StatSnapshotReq{
		TargetType:   snapshot.TargetType,
		DateType:     snapshot.DateType,
		SnapshotDate: snapshot.SnapshotDate,
	}
	got, err := ss.Get(ctx, req)
	require.NoError(t, err)
	require.Equal(t, snapshot.TotalCount, got.TotalCount)
	require.Equal(t, snapshot.NewCount, got.NewCount)
	require.Equal(t, snapshot.TrendData, got.TrendData)

	snapshot.TotalCount = 100
	snapshot.NewCount = 20
	snapshot.TrendData = database.Trend{"1": 50, "2": 50}

	err = ss.Add(ctx, snapshot)
	require.NoError(t, err)

	updateReq := types.StatSnapshotReq{
		TargetType:   snapshot.TargetType,
		DateType:     snapshot.DateType,
		SnapshotDate: snapshot.SnapshotDate,
	}

	updated, err := ss.Get(ctx, updateReq)
	require.NoError(t, err)
	require.Equal(t, 100, updated.TotalCount)
	require.Equal(t, 20, updated.NewCount)
	require.Equal(t, database.Trend{"1": 50, "2": 50}, updated.TrendData)
}

func TestStatSnapStore_QueryCumulativeCountByTime(t *testing.T) {
	type StatSnapshot struct {
		ID           int64                `bun:",pk,autoincrement" json:"id"`
		CreatedAt    time.Time            `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
		UpdatedAt    time.Time            `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
		TargetType   types.StatTargetType `bun:",notnull" json:"target_type"`
		DateType     types.StatDateType   `bun:",notnull" json:"date_type"`
		SnapshotDate string               `bun:",notnull" json:"snapshot_date"`
		TotalCount   int                  `bun:",notnull" json:"total_count"`
		NewCount     int                  `bun:",notnull" json:"new_count"`
	}
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().Truncate(time.Hour)

	users := []StatSnapshot{
		{TargetType: types.StatTargetUser, DateType: types.StatDateYear, SnapshotDate: "2025-05-01", TotalCount: 1, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
		{TargetType: types.StatTargetUser, DateType: types.StatDateMonth, SnapshotDate: "2025-05-02", TotalCount: 1, CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour)},
		{TargetType: types.StatTargetUser, DateType: types.StatDateWeek, SnapshotDate: "2025-05-03", TotalCount: 1, CreatedAt: now, UpdatedAt: now},
	}

	_, err := db.Operator.Core.NewInsert().Model(&users).Exec(ctx)
	require.NoError(t, err)

	ss := database.NewStatSnapStoreWithDB(db)

	points, err := ss.QueryCumulativeCountByTime(ctx, "stat_snapshots", "created_at", database.ByHour, 3)
	require.NoError(t, err)
	require.Len(t, points, 3)

	expectedCounts := []int{1, 2, 3} // 1 user at t-2h, 2 at t-1h, 3 at now
	for i, p := range points {
		require.Equal(t, expectedCounts[i], p.TotalCount, "unexpected count at point %d", i)
	}
}
