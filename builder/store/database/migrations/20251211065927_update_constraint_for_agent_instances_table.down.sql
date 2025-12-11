SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_type_content_id_unique_active;

ALTER TABLE agent_instances ADD CONSTRAINT unique_agent_instances_type_content_id UNIQUE (type, content_id);
