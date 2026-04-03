SET statement_timeout = 0;

--bun:split

-- data migration for add genius system agent
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
)
SELECT 
    0,  -- template_id (No template needed for system instances)
    'system', -- user_uuid (system user)
    'code',
    'system/genius-agent',
    true, -- public
    'Genius Agent',
    'General purpose assistant for complex reasoning and coding',
    true, -- built_in
    '{"system_type": "genius", "model": "", "capabilities": ["file-upload"]}'::jsonb,
    NOW(),
    NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM agent_instances 
    WHERE type = 'code' AND content_id = 'system/genius-agent'
);
