SET statement_timeout = 0;

--bun:split

ALTER TABLE account_vouchers ADD COLUMN IF NOT EXISTS voucher_type VARCHAR(255) DEFAULT 'compute';

--bun:split

ALTER TABLE account_vouchers ADD COLUMN IF NOT EXISTS voucher_name VARCHAR(255) DEFAULT '';

--bun:split

ALTER TABLE account_vouchers ADD COLUMN IF NOT EXISTS batch_no VARCHAR(255) DEFAULT '';

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_voucher_targetuuid_status_begin_date ON account_vouchers (target_uuid, status, begin_date);

CREATE INDEX IF NOT EXISTS idx_account_voucher_status_begin_date ON account_vouchers (status, begin_date);

CREATE INDEX IF NOT EXISTS idx_account_voucher_status_end_date ON account_vouchers (status, end_date);

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_voucher_batchno ON account_vouchers (batch_no);

--bun:split

UPDATE account_vouchers
SET rules = jsonb_build_object(
    'compute_limits',
    COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'cluster_ids', jsonb_build_object(
                'mode', CASE WHEN jsonb_array_length(COALESCE(elem->'cluster_ids', '[]'::jsonb)) > 0
                             THEN 'include' ELSE 'any' END,
                'values', COALESCE(elem->'cluster_ids', '[]'::jsonb)),
            'xpu_models', jsonb_build_object(
                'mode', CASE WHEN jsonb_array_length(COALESCE(elem->'xpu_models', '[]'::jsonb)) > 0
                             THEN 'include' ELSE 'any' END,
                'values', COALESCE(elem->'xpu_models', '[]'::jsonb))
        ) ORDER BY ord)
        FROM jsonb_array_elements(rules) WITH ORDINALITY AS e(elem, ord)
    ), '[]'::jsonb),
    'token_limits', jsonb_build_object(
        'models', jsonb_build_object('mode', 'any', 'values', '[]'::jsonb))
)
WHERE jsonb_typeof(rules) = 'array';
