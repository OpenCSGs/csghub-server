SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_metering_customerid_recordedat ON account_meterings (customer_id,recorded_at);
