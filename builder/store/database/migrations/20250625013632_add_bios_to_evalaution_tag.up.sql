SET statement_timeout = 0;

--bun:split

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, i18n_key, created_at, updated_at) VALUES('bias', 'evaluation', '', 'dataset', true, '偏见', 'bias', '2025-06-25 10:42:12.939', '2025-06-25 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split

INSERT INTO public.tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('oskarvanderwal', 'winogender', 'evaluation', 'bias', 'dataset', 'lm-evaluation-harness', 'hf') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO public.tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('oskarvanderwal', 'bbq', 'evaluation', 'bias', 'dataset', 'lm-evaluation-harness', 'hf') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO public.tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('oskarvanderwal', 'simple-cooccurrence-bias', 'evaluation', 'bias', 'dataset', 'lm-evaluation-harness', 'hf') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;