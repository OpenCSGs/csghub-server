SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS voucher_no VARCHAR(255);

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_statement_useruuid_voucherno ON account_statements (user_uuid, voucher_no);

--bun:split

ALTER TABLE account_bills ALTER COLUMN value SET DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS voucher_no VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS voucher_value DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS cash_value DOUBLE PRECISION DEFAULT 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution,voucher_no);

