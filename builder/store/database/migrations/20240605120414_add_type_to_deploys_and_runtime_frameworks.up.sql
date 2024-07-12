SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS type INT;
--bun:split

ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS type INT DEFAULT 1;
