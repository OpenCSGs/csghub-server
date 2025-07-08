SET statement_timeout = 0;

--bun:split

ALTER TABLE notification_settings ADD COLUMN do_not_disturb_start_time TIME WITHOUT TIME ZONE;
ALTER TABLE notification_settings ADD COLUMN do_not_disturb_end_time TIME WITHOUT TIME ZONE;

--bun:split

UPDATE notification_settings SET do_not_disturb_start_time = do_not_disturb_start::time, do_not_disturb_end_time = do_not_disturb_end::time;
