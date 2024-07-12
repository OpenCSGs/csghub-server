SET statement_timeout = 0;

--bun:split

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'mirror_tokens') THEN
        EXECUTE 'ALTER TABLE public.mirror_tokens RENAME TO sync_client_settings';
    END IF;
END $$;
