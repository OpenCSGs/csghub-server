SET statement_timeout = 0;

--bun:split

-- Migrate eligible external token price resource IDs to thirdparty://{model_id}.
-- If multiple provider resource IDs map to the same new ID, migrate only the lowest-ID row.
WITH candidates AS (
    SELECT
        id,
        'thirdparty://' || substring(resource_id FROM '^[^:]+://(.*)$') AS new_resource_id,
        row_number() OVER (
            PARTITION BY
                'thirdparty://' || substring(resource_id FROM '^[^:]+://(.*)$'),
                sku_kind,
                sku_unit_type
            ORDER BY id ASC
        ) AS rn
    FROM account_prices
    WHERE sku_type = 1
      AND sku_kind IN (4, 5)
      AND sku_unit_type = 'token'
      AND resource_id ~ '^[^:]+://.+$'
      AND resource_id NOT LIKE 'csghub://%'
      AND resource_id NOT LIKE 'thirdparty://%'
)
UPDATE account_prices ap
SET resource_id = c.new_resource_id
FROM candidates c
WHERE ap.id = c.id
  AND c.rn = 1;
