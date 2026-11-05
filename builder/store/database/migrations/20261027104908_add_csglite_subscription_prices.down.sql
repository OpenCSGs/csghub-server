SET statement_timeout = 0;

--bun:split

-- Remove only unused default-price rows; preserve subscription billing history.
DELETE FROM account_prices AS price
USING (VALUES
    ('csglite_sub_bymonth', 3900, 1, 'CSGLite 企业版月付订阅'),
    ('csglite_sub_byquarter', 8900, 3, 'CSGLite 企业版季付订阅'),
    ('csglite_sub_byyear', 33900, 12, 'CSGLite 企业版年付订阅')
) AS plan(resource_id, sku_price, sku_unit, sku_desc)
WHERE price.sku_type = 4
  AND price.sku_kind = 2
  AND price.sku_unit_type = 'month'
  AND price.sku_price_currency = 'CNY'
  AND price.discount = 1.0
  AND price.resource_id = plan.resource_id
  AND price.sku_price = plan.sku_price
  AND price.sku_unit = plan.sku_unit
  AND price.sku_desc = plan.sku_desc
  AND NOT EXISTS (
      SELECT 1 FROM account_subscriptions AS subscription
      WHERE subscription.price_id = price.id OR subscription.next_price_id = price.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM account_subscription_bills AS bill WHERE bill.price_id = price.id
  );
