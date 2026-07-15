SET statement_timeout = 0;

--bun:split

-- Drop River runtime tables before dependent functions and enum types.
DROP TABLE IF EXISTS river_notification;

--bun:split

DROP TABLE IF EXISTS river_client_queue;

--bun:split

DROP TABLE IF EXISTS river_client;

--bun:split

DROP TABLE IF EXISTS river_queue;

--bun:split

DROP TABLE IF EXISTS river_leader;

--bun:split

DROP TABLE IF EXISTS river_job;

--bun:split

DROP FUNCTION IF EXISTS river_job_notify();

--bun:split

DROP FUNCTION IF EXISTS river_job_state_in_bitmask(BIT(8), river_job_state);

--bun:split

DROP TYPE IF EXISTS river_job_state;

--bun:split

DROP TABLE IF EXISTS river_migration;
