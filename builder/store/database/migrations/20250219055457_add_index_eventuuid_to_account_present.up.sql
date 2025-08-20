SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_presents_event_uuid ON account_presents (event_uuid);
