SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_bill_date_user_scene_cusid;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_bill_date_user_scene_cusid ON account_bills (bill_date,user_uuid,scene,customer_id);

--bun:split

DROP INDEX IF EXISTS idx_account_bill_tokenid_scene_billdate;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS token_type;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS token_id;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS token_id;
