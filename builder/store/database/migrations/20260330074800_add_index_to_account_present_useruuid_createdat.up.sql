SET statement_timeout = 0;

--bun:split

-- Create composite index on user_uuid and created_at for AccountPresent table
-- This index will improve query performance for queries filtering by user_uuid and ordering by created_at
-- Using DESC order for created_at to optimize queries that fetch recent records
CREATE INDEX IF NOT EXISTS idx_account_present_useruuid_createdat ON account_presents(user_uuid, created_at DESC);