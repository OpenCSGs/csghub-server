SET statement_timeout = 0;

--bun:split

ALTER TABLE account_events DROP COLUMN IF EXISTS duplicated;

--bun:split

ALTER TABLE account_events DROP COLUMN IF EXISTS created_at;

--bun:split

DROP INDEX IF EXISTS idx_account_events_createdat;
