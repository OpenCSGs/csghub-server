SET statement_timeout = 0;

--bun:split

-- Intentionally leave the data repair in place on rollback. The up migration
-- only fills empty values, so the down migration cannot distinguish rows it
-- changed from rows that already contained the canonical translation key.
-- Clearing every matching row would destroy valid pre-existing configuration.
SELECT 1;
