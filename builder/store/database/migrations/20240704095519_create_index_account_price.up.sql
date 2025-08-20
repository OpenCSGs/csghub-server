SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_price_skutype_resid_createdat ON account_prices (sku_type,resource_id,created_at);

