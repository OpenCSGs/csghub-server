SET statement_timeout = 0;

--bun:split

-- Consolidated migration for STG/PRD:
-- 1) Introduce upstreams + routing_policy
-- 2) Migrate api_endpoint/auth_header data into upstreams when upstreams is empty
DO $$
BEGIN
    -- Ensure target columns exist.
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'llm_configs'
          AND column_name = 'upstreams'
    ) THEN
        ALTER TABLE llm_configs ADD COLUMN upstreams JSONB;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'llm_configs'
          AND column_name = 'routing_policy'
    ) THEN
        ALTER TABLE llm_configs ADD COLUMN routing_policy JSONB;
    END IF;
END $$;

--bun:split

-- Backfill upstreams from api_endpoint/auth_header when still empty.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'llm_configs'
          AND column_name = 'api_endpoint'
    ) THEN
        IF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_name = 'llm_configs'
              AND column_name = 'auth_header'
        ) THEN
            UPDATE llm_configs
            SET upstreams = jsonb_build_array(
                jsonb_strip_nulls(
                    jsonb_build_object(
                        'url', trim(api_endpoint),
                        'weight', 1,
                        'enabled', true,
                        'auth_header', NULLIF(trim(auth_header), '')
                    )
                )
            )
            WHERE (
                    upstreams IS NULL OR upstreams = '[]'::jsonb
                  )
              AND COALESCE(trim(api_endpoint), '') <> '';
        ELSE
            UPDATE llm_configs
            SET upstreams = jsonb_build_array(
                jsonb_build_object(
                    'url', trim(api_endpoint),
                    'weight', 1,
                    'enabled', true
                )
            )
            WHERE (
                    upstreams IS NULL OR upstreams = '[]'::jsonb
                  )
              AND COALESCE(trim(api_endpoint), '') <> '';
        END IF;
    END IF;
END $$;
