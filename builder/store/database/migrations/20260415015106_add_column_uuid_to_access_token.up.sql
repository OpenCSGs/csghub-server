SET statement_timeout = 0;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS ns_uuid VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS api_key VARCHAR default '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS api_key VARCHAR default '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS count DOUBLE PRECISION DEFAULT 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_access_tokens_app_ns_uuid ON access_tokens (app, ns_uuid);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_org_bill_date_org_scene_apikey_cusid ON account_bills (bill_date,user_uuid,scene,api_key,customer_id);


