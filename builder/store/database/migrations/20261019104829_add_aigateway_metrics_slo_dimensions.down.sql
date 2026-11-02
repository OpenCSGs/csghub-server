SET statement_timeout = 0;

--bun:split

-- Collapse request_model rows back onto the old (bucket_time, model,
-- provider) identity so the original primary key can be restored.  Keep the
-- row with the most requests for each group.
DELETE FROM aigateway_metrics_minute a
USING aigateway_metrics_minute b
WHERE a.bucket_time = b.bucket_time
  AND a.model = b.model
  AND a.provider = b.provider
  AND (
      a.request_total < b.request_total
      OR (a.request_total = b.request_total AND a.request_model < b.request_model)
  );

ALTER TABLE aigateway_metrics_minute
    DROP CONSTRAINT IF EXISTS aigateway_metrics_minute_pkey;
ALTER TABLE aigateway_metrics_minute
    ADD CONSTRAINT aigateway_metrics_minute_pkey
    PRIMARY KEY (bucket_time, model, provider);

--bun:split

DROP INDEX IF EXISTS idx_ame_request_model_model_provider_bucket;
DROP INDEX IF EXISTS idx_amm_request_model_model_provider_bucket;

ALTER TABLE aigateway_metrics_minute
    DROP COLUMN IF EXISTS request_model,
    DROP COLUMN IF EXISTS queue_wait_p50_ms,
    DROP COLUMN IF EXISTS queue_wait_p90_ms,
    DROP COLUMN IF EXISTS queue_depth;

-- Restore queue_wait_ms NOT NULL DEFAULT 0: rows written while the column
-- was nullable may hold NULL (request never queued) — map them back to the
-- historical 0.
UPDATE aigateway_metrics_events SET queue_wait_ms = 0 WHERE queue_wait_ms IS NULL;
ALTER TABLE aigateway_metrics_events
    ALTER COLUMN queue_wait_ms SET NOT NULL,
    ALTER COLUMN queue_wait_ms SET DEFAULT 0,
    DROP COLUMN IF EXISTS request_model,
    DROP COLUMN IF EXISTS queue_depth;
