-- Add per-request diagnostic columns to the aigateway_metrics_events table
-- (per-request raw events written by the AIGateway DBSink):
--
--   - request_id:    the gateway trace ID (from the request context), with
--                    an index so a single request can be traced end-to-end.
--   - queue_wait_ms: time the request spent waiting in an upstream admission
--                    reservation queue (0 when it was admitted immediately).
--   - error_message: for non-200 responses, a single-line, rune-safe
--                    truncated (<= 256 chars) excerpt of the upstream error
--                    body / gateway error message for troubleshooting.
--   - start_time:    the full-precision request start time. bucket_time is
--                    the minute-truncated hypertable dimension; start_time
--                    keeps the exact wall-clock moment. NULL for rows written
--                    before this column existed.
SET statement_timeout = 0;

--bun:split

ALTER TABLE aigateway_metrics_events
    ADD COLUMN IF NOT EXISTS request_id varchar NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS queue_wait_ms bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error_message varchar(256) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS start_time timestamptz;

--bun:split

-- Deploy note: a plain (non-CONCURRENTLY) build takes only seconds at the
-- observed data volume (~300k rows; TimescaleDB locks chunk-by-chunk, each
-- 1-hour chunk holds a few thousand rows), so it is safe inside the deploy
-- window. If a production table ever grows into the tens of millions, build
-- the index manually with CREATE INDEX CONCURRENTLY before deploying this
-- migration — the IF NOT EXISTS below then skips it. (bun runs plain .up.sql
-- migrations statement-by-statement outside a transaction, so CONCURRENTLY
-- is also viable inline; check your TimescaleDB version's hypertable support
-- for it first.)
CREATE INDEX IF NOT EXISTS idx_ame_request_id
    ON aigateway_metrics_events (request_id, bucket_time DESC);
