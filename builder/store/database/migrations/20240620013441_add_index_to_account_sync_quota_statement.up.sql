SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_sync_quota_sm_userid_repopath_repotype ON account_sync_quota_statements (user_id,repo_path,repo_type);
