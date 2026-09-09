SET statement_timeout = 0;

--bun:split

INSERT INTO public.tag_rules (
    namespace,
    repo_name,
    repo_type,
    category,
    tag_name,
    runtime_framework,
    source
)
VALUES (
    'opencompass',
    'AIME2025',
    'dataset',
    'evaluation',
    'examination',
    'evalscope',
    'ms'
)
ON CONFLICT (namespace, repo_name, category, runtime_framework) DO NOTHING;

--bun:split

INSERT INTO public.repository_tags (repository_id, tag_id, source)
SELECT repository.id, tag.id, 'auto'
FROM public.repositories AS repository
JOIN public.tags AS tag
  ON tag.scope = 'dataset'
 AND (
     (tag.name = 'evalscope' AND tag.category = 'runtime_framework')
     OR (tag.name = 'examination' AND tag.category = 'evaluation')
 )
WHERE LOWER(repository.path) = 'opencompass/aime2025'
   OR LOWER(repository.hf_path) = 'opencompass/aime2025'
   OR LOWER(repository.ms_path) = 'opencompass/aime2025'
   OR LOWER(repository.csg_path) = 'opencompass/aime2025'
ON CONFLICT (repository_id, tag_id) DO NOTHING;

--bun:split

DELETE FROM public.repository_tags AS repository_tag
USING public.tags AS tag
WHERE repository_tag.tag_id = tag.id
  AND tag.name = 'amd-evalscope'
  AND tag.category = 'runtime_framework'
  AND tag.scope IN ('model', 'dataset');

--bun:split

DELETE FROM public.tags
WHERE name = 'amd-evalscope'
  AND category = 'runtime_framework'
  AND scope IN ('model', 'dataset');

--bun:split

ALTER TABLE public.tag_rules DROP CONSTRAINT unique_tag_rules;
ALTER TABLE public.tag_rules
    ADD CONSTRAINT unique_tag_rules
    UNIQUE (namespace, repo_name, category);
