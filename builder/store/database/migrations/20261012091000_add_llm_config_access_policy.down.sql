SET statement_timeout = 0;

--bun:split

DROP TABLE IF EXISTS llm_config_model_grants;

--bun:split

-- CASCADE is required because catalog views (v_aigateway_catalog_models,
-- v_aigateway_enabled_models) reference llm_configs; dropping the column
-- drops those dependent views along with it.
ALTER TABLE llm_configs DROP COLUMN IF EXISTS access_policy CASCADE;
