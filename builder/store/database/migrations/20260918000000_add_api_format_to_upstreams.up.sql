SET statement_timeout = 0;

--bun:split

-- API format support: declare which protocols an upstream natively speaks and
-- whether its URL is a full endpoint URL (used as-is) or a base URL (the
-- client protocol path is appended). supported_endpoints maps a protocol
-- (chat/responses/messages) to an optional per-protocol endpoint URL; an empty
-- or null value means "use the main url field". Existing rows keep their URLs
-- untouched; we only backfill the declarations inferred from each URL's
-- terminal path so routing behavior stays identical to before the migration.
ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS is_full_url boolean NOT NULL DEFAULT false;
ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS supported_endpoints jsonb DEFAULT NULL;

--bun:split

-- Each UPDATE below is independent; the `supported_endpoints IS NULL` guard
-- makes every statement skip rows already claimed by an earlier one, so the
-- statements run in strict first-match order without falling through.

-- Terminal /chat/completions → OpenAI Chat Completions.
UPDATE ai_gateway_upstreams
SET supported_endpoints = '{"chat": null}'::jsonb, is_full_url = true
WHERE supported_endpoints IS NULL
  AND lower(regexp_replace(url, '/+$', '')) LIKE '%/chat/completions';

--bun:split

-- Terminal /responses → OpenAI Responses.
UPDATE ai_gateway_upstreams
SET supported_endpoints = '{"responses": null}'::jsonb, is_full_url = true
WHERE supported_endpoints IS NULL
  AND lower(regexp_replace(url, '/+$', '')) LIKE '%/responses';

--bun:split

-- Terminal /messages (incl. /anthropic namespace) → Anthropic Messages.
UPDATE ai_gateway_upstreams
SET supported_endpoints = '{"messages": null}'::jsonb, is_full_url = true
WHERE supported_endpoints IS NULL
  AND (lower(regexp_replace(url, '/+$', '')) LIKE '%/messages'
    OR lower(regexp_replace(url, '/+$', '')) LIKE '%/anthropic');

--bun:split

-- CSGHub-hosted deploys (source = 'csghub') keep both columns untouched:
-- their URL is a bare service root (no /v1), the sync owns it, and each
-- task family resolves its own upstream path at routing time (chat appends
-- /v1/chat/completions via the CSGHub-hosted branch, responses goes through
-- runtime version gating, multimodal handlers use adapter/default paths).
-- Declaring "chat" for a deploy that also serves video/audio would be wrong,
-- and neither full-URL nor base-URL semantics describe a bare service root.
-- Leaving supported_endpoints NULL preserves every legacy inference path
-- (CSGHub-hosted chat default, version gating, task-specific handlers).

--bun:split

-- Every other row (multimodal endpoints like /v1/images/generations,
-- /v1/audio/*, /v1/videos, and custom paths) exposes a complete endpoint URL
-- that the gateway uses as-is — none of these paths match an LLM API format,
-- so leave supported_endpoints undeclared (legacy inference/adapters apply)
-- but mark the URL as full.
UPDATE ai_gateway_upstreams
SET is_full_url = true
WHERE supported_endpoints IS NULL
  AND source <> 'csghub';
