SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories
    DROP COLUMN commercial_permission,
    DROP COLUMN compliance_status;
