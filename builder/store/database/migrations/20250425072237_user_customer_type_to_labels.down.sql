SET statement_timeout = 0;

--bun:split

ALTER TABLE users
DROP COLUMN IF EXISTS labels;

ALTER TABLE users
    ADD COLUMN customer_type INTEGER;




