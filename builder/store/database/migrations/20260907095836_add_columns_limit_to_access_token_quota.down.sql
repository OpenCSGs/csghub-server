SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_access_token_quota_token_id;

--bun:split

ALTER TABLE account_access_token_quota DROP COLUMN IF EXISTS allocation;

--bun:split

ALTER TABLE account_access_token_quota DROP COLUMN IF EXISTS token_id;
