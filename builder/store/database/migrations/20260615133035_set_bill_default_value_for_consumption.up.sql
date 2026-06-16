SET statement_timeout = 0;

--bun:split

ALTER TABLE account_bills ALTER COLUMN consumption SET DEFAULT 0;
