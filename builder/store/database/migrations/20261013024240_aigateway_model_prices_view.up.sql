SET statement_timeout = 0;

--bun:split

-- Latest enabled prices for the aigateway model scenes.
--
-- All aigateway metering resources bill under sku_type 1 (SKUCSGHub):
--   sku_kind 4 = prompt token, 5 = completion token, 7 = multimodal completion.
-- Prices form a history (new rows are appended, old ones go offline via
-- sku_status), so consumers need the latest enabled row per billing tier:
--   - token kinds (4, 5): one row per resource — the newest by created_at;
--   - multimodal kind 7: one row per (resource, unit type, resolution) tier,
--     because a resource may carry several concurrently-enabled tiers
--     (per-resolution or per-duration boundaries).
-- The two granularities are UNION'd as plain-column DISTINCT ON branches so
-- the sort keys stay index-friendly (no CASE in the ordering).
-- Consumers: the aigateway /v1/models listing filters unpriced models with
-- EXISTS over this view and enriches page rows with the price values.
DROP VIEW IF EXISTS v_aigateway_catalog_models;
DROP VIEW IF EXISTS v_aigateway_model_prices;
CREATE VIEW v_aigateway_model_prices AS
(SELECT DISTINCT ON (resource_id, sku_kind)
       resource_id, sku_kind, sku_price, sku_unit, sku_price_currency,
       sku_cached_price, sku_unit_type, resolution, sku_desc
FROM account_prices
WHERE sku_type = 1
  AND sku_kind IN (4, 5)
  AND sku_status = 1
ORDER BY resource_id, sku_kind, created_at DESC)
UNION ALL
(SELECT DISTINCT ON (resource_id, COALESCE(sku_unit_type, ''), COALESCE(resolution, ''))
       resource_id, sku_kind, sku_price, sku_unit, sku_price_currency,
       sku_cached_price, sku_unit_type, resolution, sku_desc
FROM account_prices
WHERE sku_type = 1
  AND sku_kind = 7
  AND sku_status = 1
ORDER BY resource_id, COALESCE(sku_unit_type, ''), COALESCE(resolution, ''), created_at DESC);

--bun:split

-- The aigateway model catalogue: one row per llm_config with every
-- caller-independent derivation the /v1/models listing needs, so the
-- listing query only carries request-dependent filters.
--
-- Classification mirrors llmConfigToModel in aigateway/component: the first
-- (lowest-id) csghub-sourced upstream decides; a config whose first csghub
-- upstream carries no internal model info produces no model at all and is
-- filtered out here (is_convertible), as are disabled configs.
--
-- Column semantics (all mirror the Go-side conversion):
--   provider_type   external_llm | serverless | inference
--   public_model_id the public model id (keyword search target)
--   csghub_model_id the linked csghub repo path (association)
--   tasks           comma-joined pipeline task string (task filter)
--   svc_type / secure_level / owner_uuid
--                   raw visibility inputs (private internal models are
--                   owner-only; the caller-dependent predicate stays in the
--                   listing query, views cannot take parameters)
--   is_available    at least one routable upstream (IsUpstreamUnavailable)
--   is_priced       a configured enabled price (sku_unit > 0) exists for the
--                   metering resource id of this model
DROP VIEW IF EXISTS v_aigateway_catalog_models;
CREATE VIEW v_aigateway_catalog_models AS
SELECT
    lc.*,
    CASE
        WHEN cu.id IS NULL THEN 'external_llm'
        WHEN cu.metadata->'internal_model_info'->>'svc_type' = '3' THEN 'serverless'
        ELSE 'inference'
    END AS provider_type,
    CASE WHEN cu.id IS NULL THEN lc.model_name
         ELSE cu.metadata->'internal_model_info'->>'legacy_model_id' END AS public_model_id,
    cu.metadata->'internal_model_info'->>'csghub_model_id' AS csghub_model_id,
    CASE
        WHEN cu.id IS NOT NULL THEN COALESCE(cu.metadata->'internal_model_info'->>'task', '')
        ELSE (SELECT string_agg(elem #>> '{}', ',')
              FROM jsonb_array_elements(
                  CASE WHEN jsonb_typeof(lc.metadata->'tasks') = 'array'
                      THEN lc.metadata->'tasks'
                      ELSE '[]'::jsonb END
              ) AS elem
              WHERE jsonb_typeof(elem) = 'string')
    END AS tasks,
    cu.metadata->'internal_model_info'->>'svc_type'    AS svc_type,
    cu.metadata->'internal_model_info'->>'secure_level' AS secure_level,
    cu.metadata->'internal_model_info'->>'owner_uuid'   AS owner_uuid,
    (cu.id IS NOT NULL) AS is_internal,
    (cu.id IS NULL OR cu.metadata->'internal_model_info' IS NOT NULL) AS is_convertible,
    EXISTS (
        SELECT 1 FROM ai_gateway_upstreams u
        LEFT JOIN ai_gateway_upstream_health_states h ON h.upstream_id = u.id
        LEFT JOIN ai_gateway_upstream_circuit_states c ON c.upstream_id = u.id
        WHERE u.llm_config_id = lc.id AND u.enabled
          AND (NOT u.health_check_enabled OR COALESCE(h.health_state, 'unknown') <> 'unhealthy')
          AND (NOT u.circuit_breaker_enabled OR COALESCE(c.circuit_state, 'unknown') <> 'open')
    ) AS is_available,
    EXISTS (
        SELECT 1 FROM v_aigateway_model_prices vp
        WHERE vp.resource_id = CASE
                WHEN cu.id IS NOT NULL AND cu.metadata->'internal_model_info'->>'svc_type' = '3'
                    THEN 'csghub://serverless/' || (cu.metadata->'internal_model_info'->>'csghub_model_id')
                WHEN cu.id IS NULL
                    THEN 'thirdparty://' || lc.model_name
                ELSE NULL
              END
          AND vp.sku_unit > 0
    ) AS is_priced
FROM llm_configs lc
LEFT JOIN LATERAL (
    SELECT u.* FROM ai_gateway_upstreams u
    WHERE u.llm_config_id = lc.id AND u.source = 'csghub'
    ORDER BY u.id
    LIMIT 1
) cu ON true
WHERE lc.enabled
  AND (cu.id IS NULL OR cu.metadata->'internal_model_info' IS NOT NULL);
