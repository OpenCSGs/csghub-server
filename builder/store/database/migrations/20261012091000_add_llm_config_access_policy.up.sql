SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs
    ADD COLUMN IF NOT EXISTS access_policy VARCHAR(20) NOT NULL DEFAULT 'public';

--bun:split

CREATE TABLE IF NOT EXISTS llm_config_model_grants (
    id BIGSERIAL PRIMARY KEY,
    llm_config_id BIGINT NOT NULL,
    namespace_type VARCHAR(20) NOT NULL,
    namespace VARCHAR(64) NOT NULL,
    namespace_uuid VARCHAR(64) NOT NULL,
    note TEXT NULL,
    created_by VARCHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_config_model_grant_namespace
    ON llm_config_model_grants (llm_config_id, namespace);

--bun:split

-- The gateway resolves callers to their namespace UUID (httpbase
-- GetCurrentNamespaceUUID), so enforcement looks grants up by uuid.
CREATE INDEX IF NOT EXISTS idx_llm_config_model_grant_uuid
    ON llm_config_model_grants (namespace_uuid);
