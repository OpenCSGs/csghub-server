------------------------- Users --------------------------
DROP INDEX IF EXISTS idx_users_username;

--bun:split

------------------------- Namespace --------------------------
DROP INDEX IF EXISTS idx_namespaces_path;

--bun:split

------------------------- Models --------------------------
DROP INDEX IF EXISTS idx_models_path;

--bun:split

DROP INDEX IF EXISTS idx_models_repository_id;

--bun:split

DROP INDEX IF EXISTS idx_models_user_id;

--bun:split

------------------------- Datasets --------------------------
DROP INDEX IF EXISTS idx_datasets_path;

--bun:split

DROP INDEX IF EXISTS idx_datasets_repository_id;

--bun:split

DROP INDEX IF EXISTS idx_datasets_user_id;

--bun:split

------------------------- Tags --------------------------
DROP INDEX IF EXISTS idx_tags_category;

--bun:split

DROP INDEX IF EXISTS idx_tags_name_scope;

--bun:split

DROP INDEX IF EXISTS idx_tags_scope;

--bun:split

------------------------- AccessTokens --------------------------

DROP INDEX IF EXISTS idx_access_tokens_user_id;

--bun:split

------------------------- SshKeys --------------------------

DROP INDEX IF EXISTS idx_ssh_keys_user_id;

--bun:split

------------------------- Organizations --------------------------

DROP INDEX IF EXISTS idx_organizations_path;

--bun:split

DROP INDEX IF EXISTS idx_organizations_user_id;