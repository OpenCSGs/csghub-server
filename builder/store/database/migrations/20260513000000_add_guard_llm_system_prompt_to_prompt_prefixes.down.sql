SET statement_timeout = 0;
--bun:split
DELETE FROM prompt_prefixes WHERE kind = 'guard_llm_system_prompt';
