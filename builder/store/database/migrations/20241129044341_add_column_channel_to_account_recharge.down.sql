SET statement_timeout = 0;

--bun:split

ALTER TABLE account_recharges
DROP COLUMN channel;

--bun:split

SELECT 'Rollback completed' AS status;
