SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_statement_useruuid_voucherno ON account_statements (user_uuid, voucher_no) WHERE voucher_no IS NOT NULL AND voucher_no <> '';
