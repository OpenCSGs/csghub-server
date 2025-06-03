SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS driver_version;
