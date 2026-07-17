SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_acct_stmt_daily_summary_date_user_sku_scene_cusid
    ON account_statement_daily_summaries (bill_date, user_uuid, sku_id, scene, customer_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_stmt_daily_summary_date_user
    ON account_statement_daily_summaries (bill_date, user_uuid);

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_stmt_daily_summary_date
    ON account_statement_daily_summaries (bill_date);
