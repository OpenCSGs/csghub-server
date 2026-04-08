SET statement_timeout = 0;

--bun:split

-- Add unique uuid column to namespaces for accounting instead of user/org uuid
ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS uuid VARCHAR(255);

--bun:split

Update namespaces set uuid = (select org.uuid from organizations org where org.path = namespaces.path) where namespace_type = 'organization';

--bun:split

Update namespaces set uuid = (select u.uuid from users u where u.id = namespaces.user_id) where namespace_type = 'user';

--bun:split

Update namespaces set uuid = gen_random_uuid() where uuid is null or uuid = '';

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_namespaces_uuid ON namespaces(uuid);
