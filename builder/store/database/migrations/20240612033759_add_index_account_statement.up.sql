SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_statement_event_uuid ON account_statements (event_uuid);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_statement_user_id_created_at ON account_statements (user_id, created_at);
