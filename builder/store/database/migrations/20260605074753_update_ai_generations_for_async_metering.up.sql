SET statement_timeout = 0;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS fail_reason TEXT;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS progress VARCHAR(20);

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS started_at TIMESTAMP;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS finished_at TIMESTAMP;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS upstream_id BIGINT;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS event_uuid UUID;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS metering_metadata JSONB;

--bun:split

ALTER TABLE ai_generations ADD COLUMN IF NOT EXISTS event_published_at TIMESTAMP;

--bun:split

CREATE INDEX IF NOT EXISTS idx_ai_generations_async_metering
ON ai_generations (resource_type, status, updated_at)
WHERE event_published_at IS NULL;
