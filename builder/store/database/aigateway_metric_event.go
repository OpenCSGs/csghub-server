package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
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
	// QueueWaitMs is the time the request spent waiting in an upstream
	// admission reservation queue (0 when admitted immediately).
	QueueWaitMs int64 `bun:"queue_wait_ms,notnull,default:0" json:"queue_wait_ms"`
	// ErrorMessage is a single-line, rune-safe truncated (<= 256 chars)
	// excerpt of the upstream error body / gateway error message for
	// non-200 responses. Empty for successful requests.
	ErrorMessage string `bun:"error_message,notnull,default:''" json:"error_message"`
	// RequestID is the gateway trace ID carried on the request context.
	RequestID string `bun:"request_id,notnull,default:''" json:"request_id"`
	// StartTime is the full-precision request start time (bucket_time is the
	// minute-truncated hypertable dimension). Nil for pre-existing rows.
	StartTime time.Time `bun:"start_time,nullzero" json:"start_time"`

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
	// List returns a page of raw metric events matching the query, plus the
	// total count within the time range. StartTime/EndTime must both be set
	// (zero values are rejected) so every query stays bounded by the
	// bucket_time hypertable dimension.
	List(ctx context.Context, query types.AIGatewayMetricEventQuery) ([]AIGatewayMetricEvent, int64, error)
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

// applyListFilters builds a fresh SELECT over the metric-events table with
// the caller's time range and optional filters, bound to the given model
// destination (a *[]AIGatewayMetricEvent for scanning rows, or a nil
// *AIGatewayMetricEvent for COUNT-only queries). Count and Scan each get
// their own query: bun's SelectQuery is stateful, and reusing one query
// object across a Count followed by a Scan is a known sharp edge (some bun
// versions turn it into a COUNT-only query, silently returning zero rows).
func (s *aigatewayMetricEventStoreImpl) applyListFilters(query types.AIGatewayMetricEventQuery, dest any) *bun.SelectQuery {
	q := s.db.Core.NewSelect().
		Model(dest).
		Where("ame.bucket_time >= ? AND ame.bucket_time <= ?", query.StartTime, query.EndTime)
	if query.Model != "" {
		q = q.Where("ame.model = ?", query.Model)
	}
	if query.StatusCode != 0 {
		q = q.Where("ame.status_code = ?", query.StatusCode)
	}
	if query.Username != "" {
		q = q.Where("ame.username = ?", query.Username)
	}
	if query.APIKeyMasked != "" {
		q = q.Where("ame.api_key_masked = ?", query.APIKeyMasked)
	}
	if query.RequestID != "" {
		q = q.Where("ame.request_id = ?", query.RequestID)
	}
	if len(query.UpstreamIDs) > 0 {
		q = q.Where("ame.upstream_id IN (?)", bun.In(query.UpstreamIDs))
	}
	return q
}

func (s *aigatewayMetricEventStoreImpl) List(ctx context.Context, query types.AIGatewayMetricEventQuery) ([]AIGatewayMetricEvent, int64, error) {
	if query.StartTime.IsZero() || query.EndTime.IsZero() {
		return nil, 0, errors.New("start_time and end_time are required")
	}
	if len(query.UpstreamIDs) == 0 && query.LlmConfigID > 0 {
		// LlmConfigID was set but resolved to no upstreams: no events can match.
		return nil, 0, nil
	}
	// Defensive: the admin handler always sets both, but the store must not
	// produce a negative OFFSET from zero values.
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Per <= 0 {
		query.Per = 50
	}
	// bucket_time is minute-truncated on write, while the query bounds carry
	// second precision. Truncate both bounds DOWN to the minute grid:
	// comparing raw seconds against bucket_time would exclude the entire
	// first minute's bucket (e.g. start 15:04:30 vs bucket 15:04:00). The
	// end bound over-includes at most its own trailing minute bucket, which
	// ordering by start_time keeps visually last.
	query.StartTime = query.StartTime.Truncate(time.Minute)
	query.EndTime = query.EndTime.Truncate(time.Minute)

	var events []AIGatewayMetricEvent
	total, err := s.applyListFilters(query, (*AIGatewayMetricEvent)(nil)).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count aigateway metric events: %w", err)
	}

	// StartTime is nil for pre-existing rows, so fall back to created_at.
	q := s.applyListFilters(query, &events).
		OrderExpr("COALESCE(ame.start_time, ame.created_at) DESC").
		Limit(query.Per).
		Offset((query.Page - 1) * query.Per)
	if err := q.Scan(ctx); err != nil {
		return nil, 0, fmt.Errorf("list aigateway metric events: %w", err)
	}
	return events, int64(total), nil
}
