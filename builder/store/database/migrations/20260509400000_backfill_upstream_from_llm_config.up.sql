SET statement_timeout = 0;

--bun:split

-- Backfill upstream.0.model_name from llm_configs.model_name if empty
UPDATE llm_configs
SET upstreams = jsonb_set(upstreams, '{0,model_name}', to_jsonb(model_name), true)
WHERE upstreams IS NOT NULL
  AND jsonb_array_length(upstreams) > 0
  AND (upstreams->0->>'model_name' IS NULL OR upstreams->0->>'model_name' = '');

--bun:split

-- Backfill upstream.0.provider from llm_configs.provider if empty, default to opencsg
UPDATE llm_configs
SET upstreams = jsonb_set(upstreams, '{0,provider}', to_jsonb(COALESCE(NULLIF(provider, ''), 'csghub')), true)
WHERE upstreams IS NOT NULL
  AND jsonb_array_length(upstreams) > 0
  AND (upstreams->0->>'provider' IS NULL OR upstreams->0->>'provider' = '');

--bun:split

-- Backfill upstream.0.url from llm_configs.api_endpoint if empty
UPDATE llm_configs
SET upstreams = jsonb_set(upstreams, '{0,url}', to_jsonb(api_endpoint), true)
WHERE upstreams IS NOT NULL
  AND jsonb_array_length(upstreams) > 0
  AND (upstreams->0->>'url' IS NULL OR upstreams->0->>'url' = '');

--bun:split

-- Backfill upstream.0.auth_header from llm_configs.auth_header if empty
UPDATE llm_configs
SET upstreams = jsonb_set(upstreams, '{0,auth_header}', to_jsonb(auth_header), true)
WHERE upstreams IS NOT NULL
  AND jsonb_array_length(upstreams) > 0
  AND (upstreams->0->>'auth_header' IS NULL OR upstreams->0->>'auth_header' = '');
