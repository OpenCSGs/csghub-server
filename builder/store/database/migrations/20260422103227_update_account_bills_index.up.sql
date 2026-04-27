SET statement_timeout = 0;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS token_type VARCHAR default '';

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS token_id bigint default 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS token_id bigint default 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid;

--bun:split

DROP INDEX IF EXISTS idx_unique_org_bill_date_org_scene_apikey_cusid;

--bun:split

CREATE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid ON account_bills (bill_date,user_uuid,scene,customer_id,token_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_bill_tokenid_scene_billdate ON account_bills (token_id,scene,bill_date);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_billdate_user_scene_tokenid_cusid ON account_bills (bill_date,user_uuid,scene,token_id,customer_id);
