SET statement_timeout = 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS period_id bigint DEFAULT 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bills_control ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution,voucher_no,unit_type,cluster_id,hardware_type,period_id);
