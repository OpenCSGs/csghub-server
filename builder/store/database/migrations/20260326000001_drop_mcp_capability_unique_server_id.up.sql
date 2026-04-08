SET statement_timeout = 0;
--bun:split
ALTER TABLE gateway_mcp_server_capabilities DROP CONSTRAINT IF EXISTS gateway_mcp_server_capabilities_mcp_server_id_key;