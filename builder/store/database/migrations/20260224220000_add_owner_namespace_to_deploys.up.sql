SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS owner_namespace VARCHAR(255);

--bun:split

UPDATE deploys SET owner_namespace = users.username FROM users WHERE deploys.user_id = users.id AND deploys.owner_namespace IS NULL;
