SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS container_port INT;
