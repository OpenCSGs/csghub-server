package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// AIGatewayMetricMinute stores per-minute aggregated business metrics
// collected by the Metrics Collector from Prometheus.  Each row represents
// one model+provider combination's aggregated metrics for a single minute
// bucket.
//
// This is a TimescaleDB hypertable (Apache 2-licensed features only).  The
// composite primary key (bucket_time, model, provider) enables idempotent
// upserts so the collector can safely re-process a minute after a crash or
// restart.
type AIGatewayMetricMinute struct {
	bun.BaseModel `bun:"table:aigateway_metrics_minute,alias:amm"`

	BucketTime time.Time `bun:"bucket_time,pk,notnull" json:"bucket_time"`
	Model      string    `bun:"model,pk,notnull" json:"model"`
	Provider   string    `bun:"provider,pk,notnull,default:''" json:"provider"`

	// Request counts
	RequestTotal   int64 `bun:"request_total,notnull,default:0" json:"request_total"`
	RequestSuccess int64 `bun:"request_success,notnull,default:0" json:"request_success"`
	RequestFailed  int64 `bun:"request_failed,notnull,default:0" json:"request_failed"`
	RateLimited    int64 `bun:"rate_limited,notnull,default:0" json:"rate_limited"`

	// Token consumption
	PromptTokens          int64 `bun:"prompt_tokens,notnull,default:0" json:"prompt_tokens"`
	CompletionTokens      int64 `bun:"completion_tokens,notnull,default:0" json:"completion_tokens"`
	TotalTokens           int64 `bun:"total_tokens,notnull,default:0" json:"total_tokens"`
	CachedTokens          int64 `bun:"cached_tokens,notnull,default:0" json:"cached_tokens"`
	CacheCreationTokens   int64 `bun:"cache_creation_tokens,notnull,default:0" json:"cache_creation_tokens"`

	// Latency percentiles (estimated from Prometheus histograms)
	TTFTP50Ms    float64 `bun:"ttft_p50_ms,nullzero" json:"ttft_p50_ms"`
	TTFTP90Ms    float64 `bun:"ttft_p90_ms,nullzero" json:"ttft_p90_ms"`
	LatencyP50Ms float64 `bun:"latency_p50_ms,nullzero" json:"latency_p50_ms"`
	LatencyP90Ms float64 `bun:"latency_p90_ms,nullzero" json:"latency_p90_ms"`

	// Real-time concurrency (last sample within the minute)
	ActiveRequests int64 `bun:"active_requests,notnull,default:0" json:"active_requests"`

	times
}

// AIGatewayMetricMinuteStore provides database operations for the
// aigateway_metrics_minute table.
type AIGatewayMetricMinuteStore interface {
	// UpsertBatch idempotently inserts or updates a batch of per-minute
	// metric rows.  ON CONFLICT (bucket_time, model, provider) DO UPDATE
	// ensures the collector can safely re-process a minute after a crash
	// or restart.
	UpsertBatch(ctx context.Context, rows []AIGatewayMetricMinute) error

	// QueryByTimeRange returns metric rows within [start, end), optionally
	// filtered by model (empty model = all models).
	QueryByTimeRange(ctx context.Context, start, end time.Time, model string) ([]AIGatewayMetricMinute, error)
}

type aigatewayMetricMinuteStoreImpl struct {
	db *DB
}

func NewAIGatewayMetricMinuteStore() AIGatewayMetricMinuteStore {
	return &aigatewayMetricMinuteStoreImpl{db: defaultDB}
}

func NewAIGatewayMetricMinuteStoreWithDB(db *DB) AIGatewayMetricMinuteStore {
	return &aigatewayMetricMinuteStoreImpl{db: db}
}

func (s *aigatewayMetricMinuteStoreImpl) UpsertBatch(ctx context.Context, rows []AIGatewayMetricMinute) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := s.db.Core.NewInsert().
		Model(&rows).
		On("CONFLICT (bucket_time, model, provider) DO UPDATE").
		Set("request_total = EXCLUDED.request_total").
		Set("request_success = EXCLUDED.request_success").
		Set("request_failed = EXCLUDED.request_failed").
		Set("rate_limited = EXCLUDED.rate_limited").
		Set("prompt_tokens = EXCLUDED.prompt_tokens").
		Set("completion_tokens = EXCLUDED.completion_tokens").
		Set("total_tokens = EXCLUDED.total_tokens").
		Set("cached_tokens = EXCLUDED.cached_tokens").
		Set("cache_creation_tokens = EXCLUDED.cache_creation_tokens").
		Set("ttft_p50_ms = EXCLUDED.ttft_p50_ms").
		Set("ttft_p90_ms = EXCLUDED.ttft_p90_ms").
		Set("latency_p50_ms = EXCLUDED.latency_p50_ms").
		Set("latency_p90_ms = EXCLUDED.latency_p90_ms").
		Set("active_requests = EXCLUDED.active_requests").
		Set("updated_at = current_timestamp").
		Exec(ctx)
	return err
}

func (s *aigatewayMetricMinuteStoreImpl) QueryByTimeRange(ctx context.Context, start, end time.Time, model string) ([]AIGatewayMetricMinute, error) {
	var rows []AIGatewayMetricMinute
	q := s.db.Core.NewSelect().
		Model(&rows).
		Where("bucket_time >= ?", start).
		Where("bucket_time < ?", end).
		Order("bucket_time DESC")
	if model != "" {
		q = q.Where("model = ?", model)
	}
	err := q.Scan(ctx)
	return rows, err
}
