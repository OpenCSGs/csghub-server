-- Reverse: drop all views, events hypertable, materialized view, and
-- indexes.  The base tables (aigateway_metrics_minute,
-- aigateway_metrics_checkpoints) are dropped by the Go migration down function.

-- 1. Drop Top-10 views.
DROP VIEW IF EXISTS v_aigateway_apikey_top10;
DROP VIEW IF EXISTS v_aigateway_upstream_top10;

-- 2. Drop dashboard views.
DROP VIEW IF EXISTS v_aigateway_upstream_health;
DROP VIEW IF EXISTS v_aigateway_model_distribution;
DROP VIEW IF EXISTS v_aigateway_qps_concurrency_trend;
DROP VIEW IF EXISTS v_aigateway_ttft_latency_trend;
DROP VIEW IF EXISTS v_aigateway_kpi_summary;

-- 3. Drop events hypertable.
DROP TABLE IF EXISTS aigateway_metrics_events;

-- 4. Drop hourly materialized view + unique index.
DROP MATERIALIZED VIEW IF EXISTS aigateway_metrics_hourly CASCADE;

-- 5. Drop indexes on the minute hypertable.
DROP INDEX IF EXISTS idx_amm_model_provider_bucket;
