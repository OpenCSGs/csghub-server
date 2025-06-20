SET statement_timeout = 0;

--bun:split

-- add column 'completed' to table sync_versions
ALTER TABLE sync_versions ADD COLUMN IF NOT EXISTS completed BOOLEAN DEFAULT FALSE;

