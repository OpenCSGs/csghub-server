SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_templates_csgclaw_type_name_active_unique
    ON agent_templates (type, name)
    WHERE deleted_at IS NULL AND type = 'csgclaw';
