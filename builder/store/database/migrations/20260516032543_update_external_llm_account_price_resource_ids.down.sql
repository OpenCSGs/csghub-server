SET statement_timeout = 0;

--bun:split

-- No automatic rollback: migrated rows lose the original provider prefix.
-- Restore from backup or manually map thirdparty:// rows if rollback is required.
SELECT 1;
