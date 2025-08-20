SET statement_timeout = 0;

--bun:split

ALTER TABLE prompt_prefixes ADD COLUMN IF NOT EXISTS kind VARCHAR DEFAULT 'optimize';

