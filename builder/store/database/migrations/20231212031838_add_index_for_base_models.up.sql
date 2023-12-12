------------------------- Users --------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);

--bun:split

------------------------- Namespace --------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_namespaces_path ON namespaces(path);

--bun:split

------------------------- Models --------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_models_path ON models(path);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_repository_id ON models(repository_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_models_user_id ON models(user_id);

--bun:split

------------------------- Datasets --------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_datasets_path ON datasets(path);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_datasets_repository_id ON datasets(repository_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_datasets_user_id ON datasets(user_id);

--bun:split

------------------------- Tags --------------------------
CREATE INDEX IF NOT EXISTS idx_tags_category ON tags(category);

--bun:split

CREATE INDEX IF NOT EXISTS idx_tags_scope ON tags(scope);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name_scope ON tags(name, scope);

--bun:split

------------------------- AccessTokens --------------------------

CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON access_tokens(user_id);

--bun:split

------------------------- SshKeys --------------------------

CREATE INDEX IF NOT EXISTS idx_ssh_keys_user_id ON ssh_keys(user_id);

--bun:split

------------------------- Organizations --------------------------

CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_path ON organizations(path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_organizations_user_id ON organizations(user_id);