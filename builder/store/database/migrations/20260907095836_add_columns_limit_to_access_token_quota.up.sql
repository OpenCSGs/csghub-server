SET statement_timeout = 0;

--bun:split

ALTER TABLE account_access_token_quota ADD COLUMN IF NOT EXISTS token_id bigint NOT NULL DEFAULT 0;

--bun:split

ALTER TABLE account_access_token_quota ADD COLUMN IF NOT EXISTS allocation varchar NOT NULL DEFAULT '';

--bun:split

UPDATE account_access_token_quota SET token_id = t.id FROM access_tokens t WHERE t.token = account_access_token_quota.api_key;

--bun:split

DELETE FROM account_access_token_quota where api_key not in (SELECT token FROM access_tokens);

--bun:split

Create UNIQUE Index IF NOT EXISTS idx_account_access_token_quota_token_id ON account_access_token_quota (token_id, quota_type, value_type);


