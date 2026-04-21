SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_access_tokens_app_ns_uuid;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS ns_uuid;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS api_key;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS api_key;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS count;

--bun:split

DROP INDEX IF EXISTS idx_unique_org_bill_date_org_scene_apikey_cusid;
