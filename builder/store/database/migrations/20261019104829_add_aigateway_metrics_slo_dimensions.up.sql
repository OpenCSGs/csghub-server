SET statement_timeout = 0;

-- ===========================================================================
-- AIGateway metrics: request_model dimension + queue columns (columns only).
--
-- This migration adds the columns the new metrics code needs; the PK
-- extension is included because the minute-table upsert's ON CONFLICT
-- target (bucket_time, request_model, model, provider) requires a matching
-- unique index.  Dashboard view / materialized-view rebuilds and the
-- additional time-leading index are intentionally NOT part of this migration
-- (applied out-of-band by DBA scripts; see the MR description).
--
--   events:
--     request_model   model ID the client asked for (e.g. "auto"); the
--                     existing model column remains the upstream model.
--     queue_depth     admission queue depth observed for this request;
--                     NULL unless the request entered a queue.
--     queue_wait_ms   becomes nullable (NULL = the request never queued, so
--                     queue-only statistics never mix in non-queued traffic).
--                     The DEFAULT must go too: the ORM omits NULL-marked
--                     columns on insert and the old DEFAULT 0 would silently
--                     turn them back into zeros.
--
--   minute:
--     request_model       same dimension as events.
--     queue_wait_p50_ms   admission queue wait percentiles (NULL = nothing
--     queue_wait_p90_ms   queued in that minute).
--     queue_depth         peak admission queue depth; NULL when no queueing.
--
-- Existing rows keep request_model = '' until they age out; new rows always
-- carry the requested model (falling back to the resolved model).
-- ===========================================================================

--bun:split

ALTER TABLE aigateway_metrics_events
    ADD COLUMN IF NOT EXISTS request_model varchar NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS queue_depth bigint;

ALTER TABLE aigateway_metrics_events
    ALTER COLUMN queue_wait_ms DROP NOT NULL,
    ALTER COLUMN queue_wait_ms DROP DEFAULT;

--bun:split

ALTER TABLE aigateway_metrics_minute
    ADD COLUMN IF NOT EXISTS request_model varchar NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS queue_wait_p50_ms double precision,
    ADD COLUMN IF NOT EXISTS queue_wait_p90_ms double precision,
    ADD COLUMN IF NOT EXISTS queue_depth bigint;

--bun:split

-- request_model is part of the aggregate identity: the same resolved
-- model+provider can be reached through several requested models (e.g. the
-- "auto" virtual model and a direct name), and each keeps its own minute
-- row.  The upsert conflict target depends on this primary key.
ALTER TABLE aigateway_metrics_minute
    DROP CONSTRAINT IF EXISTS aigateway_metrics_minute_pkey;
ALTER TABLE aigateway_metrics_minute
    ADD CONSTRAINT aigateway_metrics_minute_pkey
    PRIMARY KEY (bucket_time, request_model, model, provider);

--bun:split

-- Request_model-only / request_model+time queries (admin log filters, SLO
-- dashboards) are served by these leading-column indexes on both tables.
CREATE INDEX IF NOT EXISTS idx_ame_request_model_model_provider_bucket
    ON aigateway_metrics_events (request_model, model, provider, bucket_time DESC);
CREATE INDEX IF NOT EXISTS idx_amm_request_model_model_provider_bucket
    ON aigateway_metrics_minute (request_model, model, provider, bucket_time DESC);
