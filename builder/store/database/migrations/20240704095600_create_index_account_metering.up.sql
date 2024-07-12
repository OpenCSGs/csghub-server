SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_metering_event_uuid ON account_meterings (event_uuid);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_metering_userid_scene_cusid_recordedat ON account_meterings (user_uuid, scene, customer_id, recorded_at);
