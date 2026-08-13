package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// AIGatewayMetricEvent stores one raw metrics event per inference request.
// It is written synchronously by the DBSink (via a buffered channel) and
// provides the fine-grained dimensions (api_key_masked, username, upstream_id)
// that are not available in the per-minute aggregate table.
//
// Security: the raw api_key is NEVER stored. Only the masked form
// (first 4 + *** + last 4 chars) and the username are persisted.
//
// This is a TimescaleDB hypertable (Apache 2-licensed features only).
type AIGatewayMetricEvent struct {
	bun.BaseModel `bun:"table:aigateway_metrics_events,alias:ame"`

	// BucketTime is the minute-truncated timestamp of the request.  It is
	// the natural time dimension for aggregation queries.
	BucketTime time.Time `bun:"bucket_time,notnull" json:"bucket_time"`

	// Dimensions
	Model        string `bun:"model,notnull,default:''" json:"model"`
	Provider     string `bun:"provider,notnull,default:''" json:"provider"`
	UpstreamID   int64  `bun:"upstream_id,notnull,default:0" json:"upstream_id"`
	APIKeyMasked string `bun:"api_key_masked,notnull,default:''" json:"api_key_masked"`
	Username     string `bun:"username,notnull,default:''" json:"username"`

	// Request metrics
	StatusCode int    `bun:"status_code,notnull,default:0" json:"status_code"`
	IsStream   bool   `bun:"is_stream,notnull,default:false" json:"is_stream"`
	ErrorType  string `bun:"error_type,notnull,default:''" json:"error_type"`
	TTFTMs     int64  `bun:"ttft_ms,notnull,default:0" json:"ttft_ms"`
	LatencyMs  int64  `bun:"latency_ms,notnull,default:0" json:"latency_ms"`

	// Token usage
	PromptTokens        int64 `bun:"prompt_tokens,notnull,default:0" json:"prompt_tokens"`
	CompletionTokens    int64 `bun:"completion_tokens,notnull,default:0" json:"completion_tokens"`
	TotalTokens         int64 `bun:"total_tokens,notnull,default:0" json:"total_tokens"`
	CachedTokens        int64 `bun:"cached_tokens,notnull,default:0" json:"cached_tokens"`
	CacheCreationTokens int64 `bun:"cache_creation_tokens,notnull,default:0" json:"cache_creation_tokens"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// AIGatewayMetricEventStore provides database operations for the
// aigateway_metrics_events table.
type AIGatewayMetricEventStore interface {
	// BatchInsert inserts a batch of event rows.  Events are append-only
	// (no upsert) because each row represents a single request.
	BatchInsert(ctx context.Context, events []AIGatewayMetricEvent) error
}

type aigatewayMetricEventStoreImpl struct {
	db *DB
}

func NewAIGatewayMetricEventStore() AIGatewayMetricEventStore {
	return &aigatewayMetricEventStoreImpl{db: defaultDB}
}

func NewAIGatewayMetricEventStoreWithDB(db *DB) AIGatewayMetricEventStore {
	return &aigatewayMetricEventStoreImpl{db: db}
}

func (s *aigatewayMetricEventStoreImpl) BatchInsert(ctx context.Context, events []AIGatewayMetricEvent) error {
	if len(events) == 0 {
		return nil
	}
	_, err := s.db.Core.NewInsert().
		Model(&events).
		Exec(ctx)
	return err
}
