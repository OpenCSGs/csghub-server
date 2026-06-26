SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_statment_useruuid_scene_cusid_createdat ON account_statements (user_uuid, scene, customer_id, created_at);

--bun:split

DROP INDEX IF EXISTS idx_acct_statement_useruuid_voucherno;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_user_uuid_scene_cusid_evtdate;

--bun:split

DROP INDEX IF EXISTS idx_scene_created_at;

--bun:split

DROP INDEX IF EXISTS idx_user_scene_created;
