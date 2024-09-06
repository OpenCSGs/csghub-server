SET statement_timeout = 0;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS base_model VARCHAR;

