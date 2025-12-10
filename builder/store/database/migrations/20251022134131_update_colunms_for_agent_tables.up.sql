SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_instances ADD COLUMN IF NOT EXISTS built_in BOOLEAN NOT NULL DEFAULT FALSE;

--bun:split

ALTER TABLE agent_instances ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

--bun:split

ALTER TABLE agent_instance_sessions ADD COLUMN IF NOT EXISTS last_turn BIGINT NOT NULL DEFAULT 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instance_session_last_turn
ON agent_instance_sessions (last_turn);

--bun:split

ALTER TABLE agent_instance_session_histories ADD COLUMN IF NOT EXISTS turn BIGINT NOT NULL DEFAULT 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instance_session_history_session_turn_request 
ON agent_instance_session_histories (session_id, turn ASC, request DESC);

--bun:split
-- data migration for add system instances
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
-- Super Agent
(
    0,  -- template_id (No template needed for system instances)
    'system', -- user_uuid (system user)
    'code',
    'system/super-agent',
    true, -- public
    'Super Agent',
    'Advanced AI assistant with comprehensive capabilities',
    true, -- built_in
    '{"system_type": "super", "model": "", "capabilities": ["file-upload"]}'::jsonb,
    NOW(),
    NOW()
),
-- Deep Search Agent
(
    0,  -- template_id (No template needed for system instances)
    'system', -- user_uuid (system user)
    'code',
    'system/deepsearch-agent',
    true, -- public
    'Deep Search Agent',
    'Specialized agent for deep web search and information retrieval',
    true, -- built_in
    '{"system_type": "deepsearch", "model": "", "capabilities": []}'::jsonb,
    NOW(),
    NOW()
),
-- CodeSouler Agent
(
    0,  -- template_id (No template needed for system instances)
    'system', -- user_uuid (system user)
    'code',
    'system/codesouler-agent',
    true, -- public
    'CodeSouler Agent',
    'Expert coding assistant for software development tasks',
    true, -- built_in
    '{"system_type": "codesouler", "model": "", "capabilities": []}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (type, content_id) DO NOTHING;