SET statement_timeout = 0;

--bun:split

DELETE FROM tags where name = 'sglang' and category='runtime_framework';

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS task;
