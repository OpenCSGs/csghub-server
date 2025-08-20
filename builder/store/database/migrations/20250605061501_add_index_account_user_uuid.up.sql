SET
statement_timeout = 0;

--bun:split
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_user_user_uuid ON account_users (user_uuid);