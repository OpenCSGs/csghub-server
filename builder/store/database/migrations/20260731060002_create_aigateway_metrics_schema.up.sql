-- ===========================================================================
-- AIGateway Metrics Schema — TimescaleDB hypertables (Apache 2 features only),
-- standard materialized view, dashboard views, events table, and Top-10 views.
--
-- Only Apache 2-licensed TimescaleDB features are used (CREATE EXTENSION,
-- create_hypertable). TSL/enterprise features (continuous aggregates,
-- compression, retention policies) are intentionally omitted so the migration
-- runs on the Apache 2 edition.
--
-- Tables are created by the companion Go migration
-- 20260731060001_create_aigateway_metrics_tables.go.
-- ===========================================================================

-- 1. Enable TimescaleDB extension (Apache 2).
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ===========================================================================
-- 2. aigateway_metrics_minute hypertable
-- ===========================================================================

SELECT create_hypertable(
    'aigateway_metrics_minute',
    'bucket_time',
    chunk_time_interval => INTERVAL '1 hour',
    if_not_exists       => TRUE
);

-- Index for the most common dashboard query: filter by model+provider, order by time.
CREATE INDEX IF NOT EXISTS idx_amm_model_provider_bucket
    ON aigateway_metrics_minute (model, provider, bucket_time DESC);

-- Hourly rollup materialized view for long-range dashboard queries.
-- Uses a standard MATERIALIZED VIEW (not a TimescaleDB continuous aggregate,
-- which requires the TSL/enterprise license).  The view must be refreshed
-- manually via `REFRESH MATERIALIZED VIEW CONCURRENTLY aigateway_metrics_hourly`.
CREATE MATERIALIZED VIEW IF NOT EXISTS aigateway_metrics_hourly AS
SELECT
    date_trunc('hour', bucket_time)         AS bucket_time,
    model,
    provider,
    sum(request_total)                       AS request_total,
    sum(request_success)                     AS request_success,
    sum(request_failed)                      AS request_failed,
    sum(rate_limited)                        AS rate_limited,
    sum(prompt_tokens)                       AS prompt_tokens,
    sum(completion_tokens)                   AS completion_tokens,
    sum(total_tokens)                        AS total_tokens,
    sum(cached_tokens)                       AS cached_tokens,
    sum(cache_creation_tokens)               AS cache_creation_tokens,
    avg(ttft_p50_ms)                         AS ttft_p50_ms,
    avg(ttft_p90_ms)                         AS ttft_p90_ms,
    avg(latency_p50_ms)                      AS latency_p50_ms,
    avg(latency_p90_ms)                      AS latency_p90_ms,
    max(active_requests)                     AS active_requests
FROM aigateway_metrics_minute
GROUP BY 1, 2, 3
WITH NO DATA;

-- Unique index required for REFRESH MATERIALIZED VIEW CONCURRENTLY.
CREATE UNIQUE INDEX IF NOT EXISTS idx_aigateway_metrics_hourly_uidx
    ON aigateway_metrics_hourly (bucket_time, model, provider);

-- ===========================================================================
-- 3. aigateway_metrics_events hypertable (per-request raw events with
--    api_key_masked + username + upstream_id dimensions, written by DBSink).
--    Security: NEVER store the raw api_key. Only the masked form (first 4
--    + *** + last 4 chars) and the username are persisted.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS aigateway_metrics_events (
    bucket_time           timestamptz  NOT NULL,
    model                 varchar      NOT NULL DEFAULT '',
    provider              varchar      NOT NULL DEFAULT '',
    upstream_id           bigint       NOT NULL DEFAULT 0,
    api_key_masked        varchar      NOT NULL DEFAULT '',
    username              varchar      NOT NULL DEFAULT '',
    status_code           int          NOT NULL DEFAULT 0,
    is_stream             boolean      NOT NULL DEFAULT false,
    error_type            varchar      NOT NULL DEFAULT '',
    ttft_ms               bigint       NOT NULL DEFAULT 0,
    latency_ms            bigint       NOT NULL DEFAULT 0,
    prompt_tokens         bigint       NOT NULL DEFAULT 0,
    completion_tokens     bigint       NOT NULL DEFAULT 0,
    total_tokens          bigint       NOT NULL DEFAULT 0,
    cached_tokens         bigint       NOT NULL DEFAULT 0,
    cache_creation_tokens bigint       NOT NULL DEFAULT 0,
    created_at            timestamptz  NOT NULL DEFAULT current_timestamp
);

SELECT create_hypertable(
    'aigateway_metrics_events',
    'bucket_time',
    chunk_time_interval => INTERVAL '1 hour',
    if_not_exists       => TRUE
);

CREATE INDEX IF NOT EXISTS idx_ame_api_key_bucket
    ON aigateway_metrics_events (api_key_masked, bucket_time DESC);
CREATE INDEX IF NOT EXISTS idx_ame_upstream_bucket
    ON aigateway_metrics_events (upstream_id, bucket_time DESC);
CREATE INDEX IF NOT EXISTS idx_ame_model_provider_bucket
    ON aigateway_metrics_events (model, provider, bucket_time DESC);

-- ===========================================================================
-- 4. Dashboard views on aigateway_metrics_minute
-- ===========================================================================

-- 4a. KPI Summary
-- peak_concurrent_requests is the maximum per-minute active request count
-- across all minutes, computed from the events table (same overlap logic as
-- v_aigateway_qps_concurrency_trend).
CREATE VIEW v_aigateway_kpi_summary AS
WITH minute_agg AS (
    SELECT
        model,
        provider,
        sum(request_total)                       AS total_requests,
        sum(request_success)                     AS successful_requests,
        sum(request_failed)                      AS failed_requests,
        sum(rate_limited)                        AS rate_limited_requests,
        CASE
            WHEN sum(request_total) > 0
            THEN round(sum(request_success)::numeric / sum(request_total) * 100, 2)
            ELSE 0
        END                                         AS success_rate_pct,
        sum(prompt_tokens)                       AS prompt_tokens,
        sum(completion_tokens)                   AS completion_tokens,
        sum(total_tokens)                        AS total_tokens,
        sum(cached_tokens)                       AS cached_tokens,
        sum(cache_creation_tokens)               AS cache_creation_tokens,
        round(avg(ttft_p50_ms)::numeric, 2)      AS avg_ttft_p50_ms,
        round(avg(latency_p50_ms)::numeric, 2)   AS avg_latency_p50_ms
    FROM aigateway_metrics_minute
    GROUP BY model, provider
),
-- For each model+provider, compute the per-minute active request count from
-- the events table, then take the max across all minutes as the peak.
-- A request is "active" during minute T if its active window
-- [bucket_time, bucket_time + latency_ms] overlaps [T, T+1min).
peak_conc AS (
    SELECT
        m.model,
        m.provider,
        max(cnt.active_requests) AS peak_concurrent_requests
    FROM aigateway_metrics_minute m
    CROSS JOIN LATERAL (
        SELECT count(*) AS active_requests
        FROM aigateway_metrics_events ev
        WHERE ev.model = m.model
          AND ev.provider = m.provider
          -- only look at requests that started within the last 30 minutes
          AND ev.bucket_time >= m.bucket_time - interval '30 minutes'
          -- request started at or before the end of this minute
          AND ev.bucket_time < m.bucket_time + interval '1 minute'
          -- request was still running at the start of this minute
          AND ev.bucket_time + (ev.latency_ms || ' milliseconds')::interval > m.bucket_time
    ) cnt
    WHERE m.request_total > 0
    GROUP BY m.model, m.provider
)
SELECT
    ma.model,
    ma.provider,
    ma.total_requests,
    ma.successful_requests,
    ma.failed_requests,
    ma.rate_limited_requests,
    ma.success_rate_pct,
    ma.prompt_tokens,
    ma.completion_tokens,
    ma.total_tokens,
    ma.cached_tokens,
    ma.cache_creation_tokens,
    ma.avg_ttft_p50_ms,
    ma.avg_latency_p50_ms,
    COALESCE(pc.peak_concurrent_requests, 0) AS peak_concurrent_requests
FROM minute_agg ma
LEFT JOIN peak_conc pc ON ma.model = pc.model AND ma.provider = pc.provider;

-- 4b. TTFT & Latency Trend
-- Filter out rows where ttft_p50_ms is null/0 (non-stream requests or
-- minutes with no valid data) so the trend only shows meaningful TTFT points.
CREATE VIEW v_aigateway_ttft_latency_trend AS
SELECT
    bucket_time,
    model,
    provider,
    ttft_p50_ms,
    ttft_p90_ms,
    latency_p50_ms,
    latency_p90_ms
FROM aigateway_metrics_minute
WHERE ttft_p50_ms IS NOT NULL AND ttft_p50_ms > 0
ORDER BY bucket_time DESC;

-- 4c. QPS & Concurrency Trend
-- Concurrency (active_requests) is calculated directly from the events table:
-- a request is "active" during minute T if its active window
-- [bucket_time, bucket_time + latency_ms] overlaps with [T, T+1min).
-- The 30-minute lookback window bounds the scan range while ensuring long-
-- running requests (e.g. batch generation) are still counted.
CREATE VIEW v_aigateway_qps_concurrency_trend AS
SELECT
    m.bucket_time,
    m.model,
    m.provider,
    m.request_total,
    m.rate_limited,
    round(m.request_total::numeric / 60, 2)    AS qps,
    COALESCE(e.active_requests, 0)             AS active_requests
FROM aigateway_metrics_minute m
LEFT JOIN LATERAL (
    SELECT count(*) AS active_requests
    FROM aigateway_metrics_events ev
    WHERE ev.model = m.model
      AND ev.provider = m.provider
      -- only look at requests that started within the last 30 minutes
      AND ev.bucket_time >= m.bucket_time - interval '30 minutes'
      -- request started at or before the end of this minute
      AND ev.bucket_time < m.bucket_time + interval '1 minute'
      -- request was still running at the start of this minute
      AND ev.bucket_time + (ev.latency_ms || ' milliseconds')::interval > m.bucket_time
) e ON true
ORDER BY m.bucket_time DESC;

-- 4d. Model Distribution
CREATE VIEW v_aigateway_model_distribution AS
SELECT
    model,
    provider,
    sum(request_total)                       AS request_count,
    sum(total_tokens)                        AS total_tokens,
    sum(prompt_tokens)                       AS prompt_tokens,
    sum(completion_tokens)                   AS completion_tokens,
    sum(cached_tokens)                       AS cached_tokens,
    sum(cache_creation_tokens)               AS cache_creation_tokens,
    round(
        sum(total_tokens)::numeric /
        NULLIF(sum(sum(total_tokens)) OVER (), 0) * 100, 2
    )                                         AS token_share_pct,
    round(
        sum(request_total)::numeric /
        NULLIF(sum(sum(request_total)) OVER (), 0) * 100, 2
    )                                         AS request_share_pct
FROM aigateway_metrics_minute
GROUP BY model, provider;

-- 4e. Upstream Health Summary (created only if source table exists).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'ai_gateway_upstream_health_states'
    ) THEN
        CREATE OR REPLACE VIEW v_aigateway_upstream_health AS
        SELECT
            COUNT(*) FILTER (WHERE health_state = 'healthy')   AS healthy_count,
            COUNT(*) FILTER (WHERE health_state = 'degraded')  AS degraded_count,
            COUNT(*) FILTER (WHERE health_state = 'unhealthy') AS unhealthy_count,
            COUNT(*)                                            AS total_count
        FROM ai_gateway_upstream_health_states;
    END IF;
END $$;

-- ===========================================================================
-- 5. Top-10 views on aigateway_metrics_events
-- ===========================================================================

-- 5a. Top 10 API Keys by request count
-- Security: the events table stores only the masked key + username, never the
-- raw api_key.  No JOIN to access_tokens/users is needed — the display label
-- is assembled directly from the two columns.
CREATE VIEW v_aigateway_apikey_top10 AS
SELECT
    api_key_masked || COALESCE(' (' || username || ')', '') AS api_key_name,
    count(*)                                 AS request_count,
    sum(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) AS success_count,
    sum(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS failed_count,
    sum(total_tokens)                        AS total_tokens,
    sum(prompt_tokens)                       AS prompt_tokens,
    sum(completion_tokens)                   AS completion_tokens,
    round(avg(latency_ms)::numeric, 2)       AS avg_latency_ms,
    round(avg(ttft_ms)::numeric, 2)          AS avg_ttft_ms,
    min(bucket_time)                         AS first_seen,
    max(bucket_time)                         AS last_seen
FROM aigateway_metrics_events
WHERE api_key_masked != ''
GROUP BY api_key_masked, username
ORDER BY request_count DESC
LIMIT 10;

-- 5b. Top 10 Upstreams by request count
CREATE VIEW v_aigateway_upstream_top10 AS
SELECT
    upstream_id,
    model,
    provider,
    count(*)                                 AS request_count,
    sum(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) AS success_count,
    sum(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS failed_count,
    sum(total_tokens)                        AS total_tokens,
    sum(prompt_tokens)                       AS prompt_tokens,
    sum(completion_tokens)                   AS completion_tokens,
    round(avg(latency_ms)::numeric, 2)       AS avg_latency_ms,
    round(avg(ttft_ms)::numeric, 2)          AS avg_ttft_ms,
    min(bucket_time)                         AS first_seen,
    max(bucket_time)                         AS last_seen
FROM aigateway_metrics_events
WHERE upstream_id > 0
GROUP BY upstream_id, model, provider
ORDER BY request_count DESC
LIMIT 10;
