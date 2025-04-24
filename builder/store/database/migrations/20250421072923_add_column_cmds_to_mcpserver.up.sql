SET statement_timeout = 0;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS program_language VARCHAR;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS run_mode VARCHAR;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS install_deps_cmds VARCHAR;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS build_cmds VARCHAR;

--bun:split

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS launch_cmds VARCHAR;

