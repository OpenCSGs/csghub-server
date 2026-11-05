SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_voucher_batchno;

--bun:split

ALTER TABLE account_vouchers DROP COLUMN IF EXISTS voucher_type;

--bun:split

ALTER TABLE account_vouchers DROP COLUMN IF EXISTS voucher_name;

--bun:split

ALTER TABLE account_vouchers DROP COLUMN IF EXISTS batch_no;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_voucher_targettype_status_begin_date ON account_vouchers (target_type, status, begin_date);

CREATE INDEX IF NOT EXISTS idx_account_voucher_targetuuid_status_begin_date ON account_vouchers (target_uuid, status, begin_date);

CREATE INDEX IF NOT EXISTS idx_account_voucher_status_begin_date ON account_vouchers (status, begin_date);

CREATE INDEX IF NOT EXISTS idx_account_voucher_status_end_date ON account_vouchers (status, end_date);

--bun:split

UPDATE account_vouchers
SET rules = COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
        'cluster_ids', COALESCE(elem->'cluster_ids'->'values', '[]'::jsonb),
        'xpu_models',  COALESCE(elem->'xpu_models'->'values',  '[]'::jsonb)
    ) ORDER BY ord)
    FROM jsonb_array_elements(rules->'compute_limits') WITH ORDINALITY AS e(elem, ord)
), '[]'::jsonb)
WHERE jsonb_typeof(rules) = 'object';
