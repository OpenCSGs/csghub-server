SET statement_timeout = 0;

--bun:split

ALTER TABLE account_invoices DROP COLUMN IF EXISTS email_sent_at;

--bun:split

ALTER TABLE account_invoices DROP COLUMN IF EXISTS email_send_status;