SET statement_timeout = 0;

--bun:split

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'sync_client_settings') THEN
        EXECUTE 'ALTER TABLE public.sync_client_settings RENAME TO mirror_tokens';
    END IF;
END $$;
