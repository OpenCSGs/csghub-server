SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_event_uuid;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_user_uuid_created_at;

--bun:split

DROP INDEX IF EXISTS idx_account_statement_user_uuid_scene_cusid_evtdate;
