SET statement_timeout = 0;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS avatar_url VARCHAR;

