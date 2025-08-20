SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_statement_event_uuid ON account_statements (event_uuid);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_statement_user_uuid_created_at ON account_statements (user_uuid, created_at);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_statement_user_uuid_scene_cusid_evtdate ON account_statements (user_uuid,scene,customer_id,event_date);

