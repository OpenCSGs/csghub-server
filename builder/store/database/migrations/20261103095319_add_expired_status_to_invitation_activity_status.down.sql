SET statement_timeout = 0;

--bun:split

-- PostgreSQL cannot drop a single enum value; rolling back would require
-- rewriting invitation_activity_status and every dependent column, so the down
-- migration intentionally leaves 'expired' in place.
SELECT 1
