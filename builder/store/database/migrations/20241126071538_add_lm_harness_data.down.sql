SET statement_timeout = 0;

--bun:split
Delete from public.tag_rules where runtime_framework='lm-evaluation-harness';
ALTER TABLE public.tag_rules DROP COLUMN IF EXISTS namespace;
ALTER TABLE public.tag_rules DROP COLUMN IF EXISTS source;
ALTER TABLE public.tag_rules DROP CONSTRAINT unique_tag_rules;
ALTER TABLE public.tag_rules ADD CONSTRAINT unique_tag_rules UNIQUE (repo_name, category);
Delete from public.tags where name='Benchmark';
Delete from public.tags where name='lm-evaluation-harness';
