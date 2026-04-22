SET statement_timeout = 0;

--bun:split

-- Index 1: Create if not exists
CREATE INDEX IF NOT EXISTS idx_scene_created_at 
ON account_statements(scene, created_at);

--bun:split

-- Index 2: Create if not exists 
CREATE INDEX IF NOT EXISTS idx_user_scene_created 
ON account_statements(user_uuid, scene, created_at);

-- Index 3: Create if not exists 
CREATE INDEX IF NOT EXISTS idx_account_statement_group_query 
ON account_statements(user_uuid, sku_id, scene, customer_id, created_at);