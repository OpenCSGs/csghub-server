SET statement_timeout = 0;

--bun:split

DROP VIEW IF EXISTS gateway_mcp_servers_view;

--bun:split

DROP INDEX IF EXISTS idx_gateway_mcp_servers_svc_name;

--bun:split

ALTER TABLE gateway_mcp_servers
    DROP COLUMN svc_name;

--bun:split

CREATE VIEW gateway_mcp_servers_view AS
SELECT
    s.id,
    s.name,
    s.description,
    s.protocol,
    s.url,
    s.headers,
    s.env,
    s.metadata,
    s.created_at,
    s.updated_at,
    c.capabilities,
    c.status,
    c.error,
    c.refreshed_at
FROM gateway_mcp_servers s
LEFT JOIN gateway_mcp_server_capabilities c
    ON c.mcp_server_id = s.id AND c.config_hash = s.config_hash;
