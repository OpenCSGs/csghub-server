SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS customer_id VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS event_date date;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS price DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS price_unit VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS consumption DOUBLE PRECISION;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_statement_userid_scene_cusid_evtdate ON account_statements (user_id,scene,customer_id,event_date);
