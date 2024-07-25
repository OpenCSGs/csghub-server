SET statement_timeout = 0;

--bun:split

 CREATE UNIQUE INDEX IF NOT EXISTS idx_codes_repo_id_unique ON codes (repository_id);
 CREATE UNIQUE INDEX IF NOT EXISTS idx_spaces_repo_id_unique ON spaces (repository_id);

