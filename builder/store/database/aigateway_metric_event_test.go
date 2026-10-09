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

func TestAIGatewayMetricEventStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()

	// Ensure the table exists (created by migration; create_hypertable is
	// optional when TimescaleDB is absent).
	err := db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		_, err := tx.Core.NewCreateTable().
			Model((*database.AIGatewayMetricEvent)(nil)).
			IfNotExists().
			Exec(ctx)
		return err
	})
	require.NoError(t, err, "failed to ensure table exists")

	store := database.NewAIGatewayMetricEventStoreWithDB(db)
	upstreamStore := database.NewUpstreamStoreWithDB(db, nil)

	// Create an upstream so we can exercise the llm_config_id resolution.
	up := &database.Upstream{LLMConfigID: 42, URL: "http://upstream", Weight: 1, Enabled: true}
	require.NoError(t, upstreamStore.Create(ctx, up))

	base := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	events := []database.AIGatewayMetricEvent{
		{
			BucketTime: base, StartTime: base,
			Model: "kimi-k3", Provider: "p1", UpstreamID: up.ID,
			APIKeyMasked: "sk-a***1a2b", Username: "alice",
			StatusCode: 200, LatencyMs: 8200, TTFTMs: 2440, QueueWaitMs: 30,
			PromptTokens: 43551, CompletionTokens: 280, TotalTokens: 43831, CachedTokens: 100,
			CreatedAt: base,
		},
		{
			BucketTime: base.Add(2 * time.Minute), StartTime: base.Add(2 * time.Minute),
			Model: "qwen-72b", Provider: "p2", UpstreamID: up.ID,
			APIKeyMasked: "sk-b***9b1c", Username: "bob",
			StatusCode: 500, ErrorType: "upstream_error", ErrorMessage: "boom",
			LatencyMs: 1200, TTFTMs: 200,
			PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
			CreatedAt: base.Add(2 * time.Minute),
		},
		{
			// Outside the query range.
			BucketTime: base.Add(72 * time.Hour), StartTime: base.Add(72 * time.Hour),
			Model: "kimi-k3", StatusCode: 200, CreatedAt: base.Add(72 * time.Hour),
		},
	}
	require.NoError(t, store.BatchInsert(ctx, events))

	query := types.AIGatewayMetricEventQuery{
		StartTime: base.Add(-time.Minute),
		EndTime:   base.Add(time.Hour),
		Per:       10,
		Page:      1,
	}
	got, total, err := store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, got, 2)
	// Newest first.
	require.Equal(t, "qwen-72b", got[0].Model)
	require.Equal(t, "kimi-k3", got[1].Model)
	require.Equal(t, "boom", got[0].ErrorMessage)

	// Filter by model.
	query.Model = "kimi-k3"
	got, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "kimi-k3", got[0].Model)

	// Filter by status code.
	query.Model = ""
	query.StatusCode = 500
	got, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 500, got[0].StatusCode)

	// Filter by username.
	query.StatusCode = 0
	query.Username = "alice"
	got, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "alice", got[0].Username)

	// Filter by masked api key.
	query.Username = ""
	query.APIKeyMasked = "sk-a***1a2b"
	got, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "sk-a***1a2b", got[0].APIKeyMasked)

	// Filter by request id.
	query.APIKeyMasked = ""
	query.RequestID = "no-such-id"
	_, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// Pagination.
	query = types.AIGatewayMetricEventQuery{
		StartTime: base.Add(-time.Minute),
		EndTime:   base.Add(time.Hour),
		Per:       1,
		Page:      2,
	}
	got, total, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, got, 1)
	require.Equal(t, "kimi-k3", got[0].Model)

	// Missing time range is rejected.
	_, _, err = store.List(ctx, types.AIGatewayMetricEventQuery{Per: 10, Page: 1})
	require.Error(t, err)
}

func TestAIGatewayMetricEventStore_List_MinuteAlignedBounds(t *testing.T) {
	// bucket_time is minute-truncated on write; second-precision query
	// bounds must be truncated down too, or the first minute's bucket is
	// excluded (start 15:04:30 would miss bucket 15:04:00).
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	err := db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		_, err := tx.Core.NewCreateTable().
			Model((*database.AIGatewayMetricEvent)(nil)).
			IfNotExists().
			Exec(ctx)
		return err
	})
	require.NoError(t, err)

	store := database.NewAIGatewayMetricEventStoreWithDB(db)

	base := time.Date(2026, 9, 16, 15, 4, 0, 0, time.UTC)
	require.NoError(t, store.BatchInsert(ctx, []database.AIGatewayMetricEvent{
		{
			// Falls in the bucket containing the second-precision start.
			BucketTime: base, StartTime: base.Add(30 * time.Second),
			Model: "kimi-k3", StatusCode: 200, CreatedAt: base,
		},
	}))

	query := types.AIGatewayMetricEventQuery{
		// Second-precision bounds straddling the bucket.
		StartTime: base.Add(30 * time.Second),
		EndTime:   base.Add(90 * time.Second),
		Per:       10,
		Page:      1,
	}
	got, total, err := store.List(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "bucket 15:04:00 must not be excluded by start 15:04:30")
	require.Len(t, got, 1)
	require.Equal(t, "kimi-k3", got[0].Model)
}
