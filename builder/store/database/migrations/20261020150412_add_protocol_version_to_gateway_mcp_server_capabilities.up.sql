SET statement_timeout = 0;

--bun:split

ALTER TABLE gateway_mcp_server_capabilities
    ADD COLUMN IF NOT EXISTS protocol_version TEXT NOT NULL DEFAULT '';

--bun:split

CREATE OR REPLACE VIEW gateway_mcp_servers_view AS
SELECT
    s.id,
    s.name,
    s.description,
    s.protocol,
    s.url,
    s.svc_name,
    r.path AS repo_path,
    s.headers,
    s.env,
    s.metadata,
    s.created_at,
    s.updated_at,
    c.capabilities,
    c.status,
    c.error,
    c.refreshed_at,
    c.protocol_version
FROM gateway_mcp_servers s
LEFT JOIN gateway_mcp_server_capabilities c
    ON c.mcp_server_id = s.id AND c.config_hash = s.config_hash
LEFT JOIN LATERAL (
    SELECT d.repo_id
    FROM deploys d
    WHERE d.svc_name = s.svc_name
    ORDER BY d.id DESC
    LIMIT 1
) latest_deploy ON TRUE
LEFT JOIN repositories r
    ON r.id = latest_deploy.repo_id AND r.deleted_at IS NULL;
