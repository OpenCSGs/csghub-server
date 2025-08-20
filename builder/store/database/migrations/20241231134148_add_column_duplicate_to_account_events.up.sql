SET statement_timeout = 0;

--bun:split

ALTER TABLE account_events ADD COLUMN IF NOT EXISTS duplicated BOOLEAN DEFAULT false;

--bun:split

ALTER TABLE account_events ADD COLUMN IF NOT EXISTS created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_events_createdat ON account_events (created_at);


