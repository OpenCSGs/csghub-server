SET statement_timeout = 0;

--bun:split

-- Follow-up to 20260918000000 (already applied on stg): rows whose URL ends
-- in /anthropic were backfilled as FULL-URL Messages declarations, but that
-- terminal segment is a namespace prefix (DeepSeek, DashScope apps) whose
-- real endpoint is .../anthropic/v1/messages. Re-shape them into base-URL
-- Messages declarations so the gateway appends /v1/messages and keeps the
-- namespace in the URL.
UPDATE ai_gateway_upstreams
SET is_full_url = false
WHERE supported_endpoints = '{"messages": null}'::jsonb
  AND lower(regexp_replace(url, '/+$', '')) LIKE '%/anthropic';
