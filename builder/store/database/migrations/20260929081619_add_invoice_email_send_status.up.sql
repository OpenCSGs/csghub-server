SET statement_timeout = 0;

--bun:split

ALTER TABLE account_invoices ADD COLUMN IF NOT EXISTS email_send_status VARCHAR NOT NULL DEFAULT 'none';

--bun:split

ALTER TABLE account_invoices ADD COLUMN IF NOT EXISTS email_sent_at TIMESTAMP;