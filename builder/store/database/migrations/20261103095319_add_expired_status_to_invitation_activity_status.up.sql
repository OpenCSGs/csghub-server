SET statement_timeout = 0;

--bun:split

ALTER TYPE invitation_activity_status ADD VALUE IF NOT EXISTS 'expired';
