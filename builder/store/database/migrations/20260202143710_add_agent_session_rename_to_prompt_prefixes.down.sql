SET statement_timeout = 0;
--bun:split
DELETE FROM prompt_prefixes WHERE kind = 'agent_session_rename';
