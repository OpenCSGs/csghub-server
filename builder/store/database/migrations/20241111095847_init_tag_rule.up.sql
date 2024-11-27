SET statement_timeout = 0;

--bun:split

INSERT INTO public.tag_categories ("name", "scope") VALUES( 'evaluation', 'dataset') ON CONFLICT ("name", scope) DO NOTHING;

--bun:split
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Knowledge', 'evaluation', '', 'dataset', true, '知识', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Reasoning', 'evaluation', '', 'dataset', true, '推理', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Examination', 'evaluation', '', 'dataset', true, '考试', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Understanding', 'evaluation', '', 'dataset', true, '理解', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Code', 'evaluation', '', 'dataset', true, '代码', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Other', 'evaluation', '', 'dataset', true, '其他', '2024-11-11 10:42:12.939', '2024-11-11 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split
ALTER TABLE public.tag_rules ADD CONSTRAINT unique_tag_rules UNIQUE (repo_name, category);
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('wic', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('summedits', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('chid', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('afqmc', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('bustm', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cluewsc', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('wsc', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('winogrande', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('flores', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('iwslt2017', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('tydiqa', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('xcopa', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('xlsum', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('leval', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('longbench', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('govreports', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('narrativeqa', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('qasper', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('civilcomments', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('crowspairs', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cvalues', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('jigsawmultilingual', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('truthfulqa', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('advglue', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ifeval', 'evaluation', 'Other', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;

--bun:split
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('boolq', 'evaluation', 'Knowledge', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('commonsense_qa', 'evaluation', 'Knowledge', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('natural_question', 'evaluation', 'Knowledge', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('trivia_qa', 'evaluation', 'Knowledge', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;


--bun:split
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cmnli', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ocnli', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ocnli_fc', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ax-b', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ax-g', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('rte', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('anli', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('xstory_cloze', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('copa', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('record', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('hellaswag', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('piqa', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('siqa', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('math', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('gsm8k', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('theoremqa', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('strategy_qa', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('scibench', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('bbh', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('musr', 'evaluation', 'Reasoning', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;

--bun:split
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ceval-exam', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('agieval', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('mmlu', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('gaokao-bench', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cmmlu', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ai2_arc', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('xiezhi', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cmb', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('mmlu-pro', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('chembench', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('mmmlu_lite', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('wikibench', 'evaluation', 'Examination', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;

--bun:split
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('cmrc_dev', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('drcd_dev', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('race', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('openbookqa', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('squad2.0', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('lcsts', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('xsum', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('summscreen', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('eprstmt', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('lambada', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('tnews', 'evaluation', 'Understanding', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;

--bun:split
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('humaneval', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('humanevalx', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('mbpp', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('apps', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('ds1000', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('code_generation_lite', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('execution-v2', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;
INSERT INTO public.tag_rules ("repo_name", category, "tag_name", "repo_type") VALUES('test_generation', 'evaluation', 'Code', 'dataset') ON CONFLICT ("repo_name", category) DO NOTHING;