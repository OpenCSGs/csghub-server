SET statement_timeout = 0;

--bun:split

ALTER TABLE public.argo_workflows DROP COLUMN IF EXISTS cluster_id;
ALTER TABLE public.tag_rules DROP COLUMN IF EXISTS runtime_framework;
ALTER TABLE public.tag_rules DROP COLUMN IF EXISTS resource_name;

--bun:split
Delete from public.tags where name='opencompass' and category='runtime_framework';
