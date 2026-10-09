-- Reverse of 20260916094448_add_aigateway_metrics_event_queue_error.up.sql.
SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_ame_request_id;

--bun:split

ALTER TABLE aigateway_metrics_events
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS queue_wait_ms,
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS start_time;
