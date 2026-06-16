SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS voucher_no;

--bun:split

DROP INDEX IF EXISTS idx_acct_statement_useruuid_voucherno;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS voucher_no;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS voucher_value;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS cash_value;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution);
