SET statement_timeout = 0;

--bun:split

ALTER TABLE notification_settings DROP COLUMN do_not_disturb_start_time;
ALTER TABLE notification_settings DROP COLUMN do_not_disturb_end_time;
