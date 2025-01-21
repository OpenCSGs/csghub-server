SET statement_timeout = 0;

--bun:split
DELETE FROM public.tags where name = 'sglang' and category='runtime_framework';
ALTER TABLE deploys DROP COLUMN task;
