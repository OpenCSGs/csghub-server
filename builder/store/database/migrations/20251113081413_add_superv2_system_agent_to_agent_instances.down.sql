SET statement_timeout = 0;

--bun:split

-- delete superv2 system agent
DELETE FROM agent_instances WHERE built_in = true AND content_id = 'system/superv2-agent' AND type = 'code';