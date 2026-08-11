package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

// TestAIGatewayMetricMinuteStore_UpsertBatch verifies the idempotent upsert
// logic: inserting the same (bucket_time, model, provider) twice should update
// rather than duplicate.
//
// NOTE: This test requires the aigateway_metrics_minute table to exist.
// The table is created by the Go migration (create_aigateway_metrics_minute)
// which uses bun's createTables helper.  The migration uses only Apache
// 2-licensed TimescaleDB features (create_hypertable); if the test DB does
// not have the timescaledb extension, the base table is still created as a
// regular PostgreSQL table.
func TestAIGatewayMetricMinuteStore_UpsertBatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()

	// Ensure the table exists (the base table is created by the migration's
	// createTables helper; create_hypertable is optional).
	err := db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		_, err := tx.Core.NewCreateTable().
			Model((*database.AIGatewayMetricMinute)(nil)).
			IfNotExists().
			Exec(ctx)
		return err
	})
	require.NoError(t, err, "failed to ensure table exists")

	store := database.NewAIGatewayMetricMinuteStoreWithDB(db)

	bucket := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

	// First insert.
	rows := []database.AIGatewayMetricMinute{
		{
			BucketTime:         bucket,
			Model:              "qwen-72b",
			Provider:           "provider-a",
			RequestTotal:       100,
			RequestSuccess:     95,
			RequestFailed:      5,
			PromptTokens:       500,
			CompletionTokens:   200,
			TotalTokens:        700,
			CachedTokens:       150,
			CacheCreationTokens: 80,
			TTFTP50Ms:          180.5,
			ActiveRequests:     12,
		},
	}
	err = store.UpsertBatch(ctx, rows)
	require.NoError(t, err)

	// Verify.
	result, err := store.QueryByTimeRange(ctx, bucket, bucket.Add(time.Minute), "qwen-72b")
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(100), result[0].RequestTotal)
	require.Equal(t, "qwen-72b", result[0].Model)
	require.Equal(t, "provider-a", result[0].Provider)
	require.Equal(t, int64(150), result[0].CachedTokens)
	require.Equal(t, int64(80), result[0].CacheCreationTokens)

	// Second insert with updated values (idempotent upsert).
	rows[0].RequestTotal = 150
	rows[0].TTFTP50Ms = 200.0
	err = store.UpsertBatch(ctx, rows)
	require.NoError(t, err)

	// Verify updated values.
	result, err = store.QueryByTimeRange(ctx, bucket, bucket.Add(time.Minute), "qwen-72b")
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(150), result[0].RequestTotal)
	require.Equal(t, 200.0, result[0].TTFTP50Ms)

	// Cleanup.
	_, _ = db.Core.NewDelete().Model((*database.AIGatewayMetricMinute)(nil)).Where("bucket_time = ?", bucket).Exec(ctx)
}

func TestAIGatewayMetricMinuteStore_QueryByTimeRange_AllModels(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()

	err := db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		_, err := tx.Core.NewCreateTable().
			Model((*database.AIGatewayMetricMinute)(nil)).
			IfNotExists().
			Exec(ctx)
		return err
	})
	require.NoError(t, err)

	store := database.NewAIGatewayMetricMinuteStoreWithDB(db)

	bucket := time.Date(2026, 7, 31, 13, 0, 0, 0, time.UTC)
	rows := []database.AIGatewayMetricMinute{
		{BucketTime: bucket, Model: "model-a", Provider: "provider-a", RequestTotal: 10},
		{BucketTime: bucket, Model: "model-b", Provider: "provider-b", RequestTotal: 20},
	}
	err = store.UpsertBatch(ctx, rows)
	require.NoError(t, err)

	// Query all models (empty model filter).
	result, err := store.QueryByTimeRange(ctx, bucket, bucket.Add(time.Minute), "")
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Cleanup.
	_, _ = db.Core.NewDelete().Model((*database.AIGatewayMetricMinute)(nil)).Where("bucket_time = ?", bucket).Exec(ctx)
}

func TestAIGatewayMetricMinuteStore_UpsertBatch_Empty(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAIGatewayMetricMinuteStoreWithDB(db)
	err := store.UpsertBatch(context.TODO(), nil)
	require.NoError(t, err)
}
