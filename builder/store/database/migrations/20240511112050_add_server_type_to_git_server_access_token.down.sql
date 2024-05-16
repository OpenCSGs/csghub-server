SET statement_timeout = 0;

--bun:split

ALTER TABLE git_server_access_tokens DROP COLUMN IF EXISTS server_type;

