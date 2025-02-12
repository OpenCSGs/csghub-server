SET statement_timeout = 0;

--bun:split
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('sglang', 'runtime_framework', 'inference', 'model', true, 'SGLang') ON CONFLICT ("name", category, scope) DO NOTHING;
ALTER TABLE deploys ADD COLUMN task VARCHAR(255);
