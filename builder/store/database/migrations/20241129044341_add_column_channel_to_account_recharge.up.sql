SET statement_timeout = 0;

--bun:split

-- 1. add channel to account_recharges
ALTER TABLE account_recharges
    ADD COLUMN channel VARCHAR;

--bun:split

-- 2. add payment_payment's channel copy to account_recharges.channel
UPDATE account_recharges ar
SET channel = pp.channel
    FROM payment_payment pp
WHERE ar.payment_uuid = pp.payment_uuid;

--bun:split

-- 3. 确认数据迁移完成
SELECT 'Data migration completed' AS status;
