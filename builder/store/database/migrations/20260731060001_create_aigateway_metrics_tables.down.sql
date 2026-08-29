-- Reverse: drop the base tables created by the companion up migration.
-- TimescaleDB hypertable conversion, views, and policies are dropped by
-- 20260731060002_create_aigateway_metrics_schema.down.sql.

DROP TABLE IF EXISTS aigateway_metrics_checkpoints;
DROP TABLE IF EXISTS aigateway_metrics_minute;
