SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_ai_generations_async_metering;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS event_published_at;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS metering_metadata;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS event_uuid;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS upstream_id;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS finished_at;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS started_at;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS progress;

--bun:split

ALTER TABLE ai_generations DROP COLUMN IF EXISTS fail_reason;
