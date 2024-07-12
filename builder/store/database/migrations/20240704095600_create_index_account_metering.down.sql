SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_metering_event_uuid;

--bun:split

DROP INDEX IF EXISTS idx_account_metering_userid_scene_cusid_recordedat;
