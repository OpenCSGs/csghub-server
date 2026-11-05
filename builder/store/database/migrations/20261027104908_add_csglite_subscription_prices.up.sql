SET statement_timeout = 0;

--bun:split

-- Prices are total amounts in fen for each billing period. Existing plans
-- remain administrator-managed and must not be overwritten on reapplication.
INSERT INTO account_prices (
    sku_type, sku_price, sku_unit, sku_desc, resource_id,
    created_at, updated_at, sku_unit_type, sku_price_currency,
    sku_kind, quota, discount, use_limit_price, resolution,
    sku_status, sku_cached_price
)
SELECT 4, plan.sku_price, plan.sku_unit, plan.sku_desc, plan.resource_id,
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'month', 'CNY',
       2, '', 1.0, 0, '', 1, -1
FROM (VALUES
    ('csglite_sub_bymonth', 3900, 1, 'CSGLite 企业版月付订阅'),
    ('csglite_sub_byquarter', 8900, 3, 'CSGLite 企业版季付订阅'),
    ('csglite_sub_byyear', 33900, 12, 'CSGLite 企业版年付订阅')
) AS plan(resource_id, sku_price, sku_unit, sku_desc)
WHERE NOT EXISTS (
    SELECT 1 FROM account_prices AS existing
    WHERE existing.sku_type = 4
      AND existing.sku_kind = 2
      AND existing.sku_unit_type = 'month'
      AND existing.resource_id = plan.resource_id
);
