SET statement_timeout = 0;

--bun:split

UPDATE tag_rules SET tag_name = LOWER(tag_name);

--bun:split
INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('evalscope', 'runtime_framework', 'evaluation', 'model', true, 'evalscope') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('evalscope', 'runtime_framework', 'evaluation', 'dataset', true, 'evalscope') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split

-- Generated SQL statements for dataset tags
INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('HuggingFaceH4', 'aime_2024', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('opencompass', 'AIME2025', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'alpaca_eval', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'ai2_arc', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'arena-hard-auto-v0.1', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'bbh', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'ceval-exam', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'Chinese-SimpleQA', 'evaluation', 'knowledge', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'cmmlu', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'competition_math', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('yale-nlp', 'DocMath-Eval', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'DROP', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('iic', 'frames', 'evaluation', 'understanding', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'gpqa', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'gsm8k', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'hellaswag', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'humaneval', 'evaluation', 'code', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'ifeval', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'iquiz', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'code_generation_lite', 'evaluation', 'code', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'MATH-500', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('HiDolphin', 'MaritimeBench', 'evaluation', 'knowledge', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'mmlu', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'mmlu-pro', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'mmlu-redux-2.0', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'MuSR', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'Needle-in-a-Haystack-Corpus', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('Qwen', 'ProcessBench', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'race', 'evaluation', 'knowledge', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'SimpleQA', 'evaluation', 'knowledge', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('m-a-p', 'SuperGPQA', 'evaluation', 'examination', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'ToolBench-Statich', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'trivia_qa', 'evaluation', 'knowledge', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('modelscope', 'truthful_qa', 'evaluation', 'other', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;

INSERT INTO tag_rules (namespace, "repo_name", category, "tag_name", "repo_type", runtime_framework, source) VALUES ('AI-ModelScope', 'winogrande_val', 'evaluation', 'reasoning', 'dataset', 'evalscope', 'ms') ON CONFLICT (namespace, "repo_name", category) DO NOTHING;
