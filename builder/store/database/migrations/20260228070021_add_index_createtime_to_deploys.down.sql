SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_deploys_created_at;

--bun:split

DROP INDEX IF EXISTS idx_argoworkflows_submit_at;
