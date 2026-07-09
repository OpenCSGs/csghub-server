SET statement_timeout = 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(100) DEFAULT '';

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS hardware_type VARCHAR(50) DEFAULT '';

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_billdate_clusterid ON account_bills (bill_date, cluster_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_billdate_hardware ON account_bills (bill_date, hardware_type);

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_billdate_scene ON account_bills (bill_date, scene);

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution,voucher_no,unit_type,cluster_id,hardware_type);

