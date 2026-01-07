SET statement_timeout = 0;

--bun:split

-- delete csghub-docs system agent
DELETE FROM agent_instances WHERE built_in = true AND content_id = 'system/csghub-docs-agent' AND type = 'code';
