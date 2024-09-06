SET statement_timeout = 0;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS base_model;

