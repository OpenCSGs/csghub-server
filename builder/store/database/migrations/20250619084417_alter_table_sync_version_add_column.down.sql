SET statement_timeout = 0;

--bun:split

-- drop coloumn 'completed' from table sync_versions
ALTER TABLE sync_versions DROP COLUMN IF EXISTS completed;

