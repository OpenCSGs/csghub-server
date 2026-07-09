SET statement_timeout = 0;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS cluster_id;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS hardware_type;

--bun:split

DROP INDEX IF EXISTS idx_acct_billdate_clusterid;

--bun:split

DROP INDEX IF EXISTS idx_acct_billdate_hardware;

--bun:split

DROP INDEX IF EXISTS idx_acct_billdate_scene;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid_tokenid_type_res ON account_bills (bill_date,user_uuid,scene,customer_id,token_id,data_type,resolution,voucher_no,unit_type);
