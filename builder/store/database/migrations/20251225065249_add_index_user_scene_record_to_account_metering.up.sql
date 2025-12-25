SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_meter_user_scene_recordat ON account_meterings (user_uuid, scene, recorded_at);
