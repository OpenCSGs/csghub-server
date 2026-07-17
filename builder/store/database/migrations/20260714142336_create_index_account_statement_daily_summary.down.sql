SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_acct_stmt_daily_summary_date;

--bun:split

DROP INDEX IF EXISTS idx_acct_stmt_daily_summary_date_user;

--bun:split

DROP INDEX IF EXISTS idx_unique_acct_stmt_daily_summary_date_user_sku_scene_cusid;
