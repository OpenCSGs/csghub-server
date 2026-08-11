-- Create the aigateway_metrics_minute and aigateway_metrics_checkpoints base
-- tables (plain tables; the TimescaleDB hypertable + views are created by the
-- companion SQL migration 20260731060002_create_aigateway_metrics_schema).

-- 1. aigateway_metrics_minute: per-minute aggregated business metrics.
--    Composite PK (bucket_time, model, provider) enables idempotent upserts.
CREATE TABLE IF NOT EXISTS aigateway_metrics_minute (
    bucket_time           timestamptz  NOT NULL,
    model                 varchar      NOT NULL,
    provider              varchar      NOT NULL DEFAULT '',

    -- Request counts
    request_total         bigint       NOT NULL DEFAULT 0,
    request_success       bigint       NOT NULL DEFAULT 0,
    request_failed        bigint       NOT NULL DEFAULT 0,
    rate_limited          bigint       NOT NULL DEFAULT 0,

    -- Token consumption
    prompt_tokens         bigint       NOT NULL DEFAULT 0,
    completion_tokens     bigint       NOT NULL DEFAULT 0,
    total_tokens          bigint       NOT NULL DEFAULT 0,
    cached_tokens         bigint       NOT NULL DEFAULT 0,
    cache_creation_tokens bigint       NOT NULL DEFAULT 0,

    -- Latency percentiles (nullzero — null when no data)
    ttft_p50_ms           double precision,
    ttft_p90_ms           double precision,
    latency_p50_ms        double precision,
    latency_p90_ms        double precision,

    -- Real-time concurrency (last sample within the minute)
    active_requests       bigint       NOT NULL DEFAULT 0,

    created_at            timestamptz  NOT NULL DEFAULT current_timestamp,
    updated_at            timestamptz  NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY (bucket_time, model, provider)
);

-- 2. aigateway_metrics_checkpoints: watermark for the metrics collector.
CREATE TABLE IF NOT EXISTS aigateway_metrics_checkpoints (
    job_name     varchar      NOT NULL,
    last_minute  timestamptz  NOT NULL,
    created_at   timestamptz  NOT NULL DEFAULT current_timestamp,
    updated_at   timestamptz  NOT NULL DEFAULT current_timestamp,
    PRIMARY KEY (job_name)
);
