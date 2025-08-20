SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid ON account_bills (bill_date,user_id,scene,customer_id);

