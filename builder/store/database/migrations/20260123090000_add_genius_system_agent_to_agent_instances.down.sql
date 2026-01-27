SET statement_timeout = 0;

--bun:split

-- delete genius system agent
DELETE FROM agent_instances WHERE built_in = true AND content_id = 'system/genius-agent' AND type = 'code';
