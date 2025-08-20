SET statement_timeout = 0;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS avatar_url;
