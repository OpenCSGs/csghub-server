SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_statement_event_uuid;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_user_id_created_at;
