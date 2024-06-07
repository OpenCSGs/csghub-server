SET statement_timeout = 0;

--bun:split

ALTER TABLE events ADD COLUMN IF NOT EXISTS extension TEXT;
