SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS data_type VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS resolution VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS duration DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS data_type VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS resolution VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS duration DOUBLE PRECISION DEFAULT 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid;

--bun:split

CREATE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution);

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS resolution VARCHAR(255) DEFAULT '';
