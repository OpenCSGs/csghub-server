SET statement_timeout = 0;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS program_language;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS run_mode;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS install_deps_cmds;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS build_cmds;

--bun:split

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS launch_cmds;

