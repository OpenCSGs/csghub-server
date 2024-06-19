SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS customer_id;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS event_date;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS consumption;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS price_unit;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS price;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_userid_scene_cusid_evtdate;
