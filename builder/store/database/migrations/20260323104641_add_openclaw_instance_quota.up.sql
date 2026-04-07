SET statement_timeout = 0;

--bun:split
UPDATE agent_configs
SET config = config || '{"openclaw_instance_quota_per_user": 1}'::jsonb
WHERE name = 'instance';
