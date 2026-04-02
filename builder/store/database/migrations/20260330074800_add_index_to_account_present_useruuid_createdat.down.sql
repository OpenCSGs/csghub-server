SET statement_timeout = 0;

--bun:split

-- Drop the composite index on user_uuid and created_at for AccountPresent table
DROP INDEX IF EXISTS idx_account_present_useruuid_createdat;