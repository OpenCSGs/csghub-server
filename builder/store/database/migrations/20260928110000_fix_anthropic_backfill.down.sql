SET statement_timeout = 0;

--bun:split

-- Restore the pre-20260928110000 shape: /anthropic-terminated rows were
-- backfilled as full-URL Messages declarations.
UPDATE ai_gateway_upstreams
SET is_full_url = true
WHERE supported_endpoints = '{"messages": null}'::jsonb
  AND lower(regexp_replace(url, '/+$', '')) LIKE '%/anthropic';
