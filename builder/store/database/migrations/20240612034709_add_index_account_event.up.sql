SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_events_event_uuid ON account_events (event_uuid);
