SET statement_timeout = 0;

--bun:split

ALTER TABLE public.tag_rules DROP CONSTRAINT unique_tag_rules;
ALTER TABLE public.tag_rules
    ADD CONSTRAINT unique_tag_rules
    UNIQUE (namespace, repo_name, category, runtime_framework);

--bun:split

INSERT INTO public.tags (name, category, "group", scope, built_in, show_name)
VALUES
    ('amd-evalscope', 'runtime_framework', 'evaluation', 'model', true, 'amd-evalscope'),
    ('amd-evalscope', 'runtime_framework', 'evaluation', 'dataset', true, 'amd-evalscope')
ON CONFLICT (name, category, scope) DO NOTHING;

--bun:split

DELETE FROM public.repository_tags AS repository_tag
USING public.repositories AS repository, public.tags AS tag
WHERE repository_tag.repository_id = repository.id
  AND repository_tag.tag_id = tag.id
  AND tag.scope = 'dataset'
  AND (
      (tag.name = 'evalscope' AND tag.category = 'runtime_framework')
      OR (tag.name = 'examination' AND tag.category = 'evaluation')
  )
  AND (
      LOWER(repository.path) = 'opencompass/aime2025'
      OR LOWER(repository.hf_path) = 'opencompass/aime2025'
      OR LOWER(repository.ms_path) = 'opencompass/aime2025'
      OR LOWER(repository.csg_path) = 'opencompass/aime2025'
  );

--bun:split

DELETE FROM public.tag_rules
WHERE namespace = 'opencompass'
  AND LOWER(repo_name) = 'aime2025'
  AND category = 'evaluation'
  AND runtime_framework = 'evalscope';
