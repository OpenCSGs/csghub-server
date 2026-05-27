SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS resolution;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS duration;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS data_type;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS resolution;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS duration;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS data_type;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid ON account_bills (bill_date,user_uuid,scene,customer_id,token_id);

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS resolution;
