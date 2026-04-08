SET statement_timeout = 0;

--bun:split

-- data migration for add superv2 system agent
INSERT INTO agent_instances (
    template_id,    
    user_uuid,
    type,
    content_id,
    public,
    name,
    description,
    built_in,
    metadata,
    created_at,
    updated_at
) VALUES 
-- Superv2 System Agent
(
    0,  -- template_id (No template needed for system instances)
    'system', -- user_uuid (system user)
    'code',
    'system/superv2-agent',
    true, -- public
    'Superv2 Agent',
    'Advanced AI assistant v2 with comprehensive capabilities',
    true, -- built_in
    '{"system_type": "superv2", "model": "", "capabilities": ["file-upload"]}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (type, content_id) DO NOTHING;