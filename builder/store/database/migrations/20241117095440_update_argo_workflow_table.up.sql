SET statement_timeout = 0;

--bun:split

ALTER TABLE public.argo_workflows ADD COLUMN IF NOT EXISTS cluster_id VARCHAR;
ALTER TABLE public.argo_workflows ADD COLUMN IF NOT EXISTS namespace VARCHAR;
ALTER TABLE public.argo_workflows ADD COLUMN IF NOT EXISTS resource_name VARCHAR;

--bun:split

--add opencompass as runtime framework for all tag_rules created in 20241111095847_init_tag_rule.up.sql
--then drop the default the value
ALTER TABLE public.tag_rules ADD COLUMN IF NOT EXISTS runtime_framework VARCHAR DEFAULT 'opencompass';
ALTER TABLE public.tag_rules ALTER COLUMN runtime_framework DROP DEFAULT;

--bun:split
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('opencompass', 'runtime_framework', 'evaluation', 'model', true, 'OpenCompass') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('opencompass', 'runtime_framework', 'evaluation', 'dataset', true, 'OpenCompass') ON CONFLICT ("name", category, scope) DO NOTHING;
