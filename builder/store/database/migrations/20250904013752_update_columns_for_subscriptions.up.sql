SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS discount DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS discount DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS regular_value DOUBLE PRECISION;

--bun:split

ALTER TABLE account_subscription_bills ADD COLUMN IF NOT EXISTS sku_type BIGINT;

--bun:split

ALTER TABLE account_subscription_bills ADD COLUMN IF NOT EXISTS discount DOUBLE PRECISION;

--bun:split

ALTER TABLE account_subscription_usages ADD COLUMN IF NOT EXISTS value_type BIGINT;

--bun:split

ALTER TABLE account_subscription_usages ADD COLUMN IF NOT EXISTS sku_type BIGINT;

--bun:split

DROP INDEX IF EXISTS idx_account_subscription_useruuid_skutype;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_subscription_useruuid_skutype ON account_subscriptions (user_uuid,sku_type);

--bun:split

DROP INDEX IF EXISTS idx_account_subscription_bill_createdat_useruuid_status;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_subscription_bill_createdat_useruuid_status_skutype ON account_subscription_bills (created_at,user_uuid,status,sku_type);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_sub_usage_billid_useruuid_skutype ON account_subscription_usages (bill_id,user_uuid,sku_type);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_sub_usage_billmonth_useruuid_skutype ON account_subscription_usages (bill_month,user_uuid,sku_type);

