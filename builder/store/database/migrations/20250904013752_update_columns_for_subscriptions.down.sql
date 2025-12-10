SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS discount;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS discount;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS regular_value;

--bun:split

ALTER TABLE account_subscription_bills DROP COLUMN IF EXISTS sku_type;

--bun:split

ALTER TABLE account_subscription_bills DROP COLUMN IF EXISTS discount;

--bun:split

ALTER TABLE account_subscription_usages DROP COLUMN IF EXISTS value_type;

--bun:split

ALTER TABLE account_subscription_usages DROP COLUMN IF EXISTS sku_type;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_subscription_useruuid_skutype;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_subscription_useruuid_skutype ON account_subscriptions (user_uuid,sku_type);

--bun:split

DROP INDEX IF EXISTS idx_account_subscription_bill_createdat_useruuid_status_skutype;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_subscription_bill_createdat_useruuid_status ON account_subscription_bills (created_at,user_uuid,status);

--bun:split

DROP INDEX IF EXISTS idx_account_sub_usage_billid_useruuid_skutype;

--bun:split

DROP INDEX IF EXISTS idx_account_sub_usage_billmonth_useruuid_skutype;
