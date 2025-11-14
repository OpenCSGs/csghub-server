SET statement_timeout = 0;

--bun:split

-- Generated SQL statements for MS-SWIFT dataset tags
-- Source: https://swift.readthedocs.io/en/latest/Instruction/Supported-models-and-datasets.html#datasets

INSERT INTO tags (name, category, "group", scope, built_in, show_name) VALUES ('ms-swift', 'task', 'finetune', 'dataset', false, 'ms-swift') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-MO', 'NuminaMath-1.5', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-MO', 'NuminaMath-CoT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-MO', 'NuminaMath-TIR', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'COIG-CQIA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'CodeAlpaca-20k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'DISC-Law-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'DISC-Med-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'Duet-v0.5', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'GuanacoDataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'LLaVA-Instruct-150K', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'LLaVA-Pretrain', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'LaTeX_OCR', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'LongAlpaca-12k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'M3IT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'MATH-lighteval', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'Magpie-Qwen2-Pro-200K-Chinese', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'Magpie-Qwen2-Pro-200K-English', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'Magpie-Qwen2-Pro-300K-Filtered', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'MathInstruct', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'MovieChat-1K-test', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'Open-Platypus', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'OpenO1-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'OpenOrca', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'OpenOrca-Chinese', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'SFT-Nectar', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'ShareGPT-4o', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'ShareGPT4V', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'SkyPile-150B', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'WizardLM_evol_instruct_V2_196k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'alpaca-cleaned', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'alpaca-gpt4-data-en', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'alpaca-gpt4-data-zh', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'blossom-math-v2', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'captcha-images', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'chartqa_digit_r1v_format', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'clevr_cogen_a_train', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'coco', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'databricks-dolly-15k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'deepctrl-sft-data', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'egoschema', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'firefly-train-1.1M', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'function-calling-chatml', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'generated_chat_0.4M', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'guanaco_belle_merge_v1.0', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'hh-rlhf', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'hh_rlhf_cn', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'lawyer_llama_data', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'leetcode-solutions-python', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'lmsys-chat-1m', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'math-trn-format', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'ms_agent_for_agentfabric', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'orpo-dpo-mix-40k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'pile', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'ruozhiba', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'school_math_0.25M', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'sharegpt_gpt4', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'sql-create-context', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'stack-exchange-paired', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'starcoderdata', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'synthetic_text_to_sql', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'texttosqlv2_25000_v2', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'the-stack', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'tigerbot-law-plugin', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'train_0.5M_CN', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'train_1M_CN', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'train_2M_CN', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'tulu-v2-sft-mixture', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'ultrafeedback-binarized-preferences-cleaned-kto', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'webnovel_cn', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'wikipedia-cn-20230720-filtered', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('AI-ModelScope', 'zhihu_rlhf_3k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('DAMO_NLP', 'jd', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('FreedomIntelligence', 'medical-o1-reasoning-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('HumanLLMs', 'Human-Like-DPO-Dataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('LLM-Research', 'xlam-function-calling-60k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('MTEB', 'scidocs-reranking', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('MTEB', 'stackoverflowdupquestions-reranking', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('OmniData', 'Zhihu-KOL', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('OmniData', 'Zhihu-KOL-More-Than-100-Upvotes', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('PowerInfer', 'LONGCOT-Refine-500K', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('PowerInfer', 'QWQ-LONGCOT-500K', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('ServiceNow-AI', 'R1-Distill-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('TIGER-Lab', 'MATH-plus', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('Tongyi-DataEngine', 'SA1B-Dense-Caption', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('Tongyi-DataEngine', 'SA1B-Paired-Captions-Images', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('YorickHe', 'CoT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('YorickHe', 'CoT_zh', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('ZhipuAI', 'LongWriter-6k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('bespokelabs', 'Bespoke-Stratos-17k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('codefuse-ai', 'CodeExercise-Python-27k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('codefuse-ai', 'Evol-instruction-66k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('damo', 'MSAgent-Bench', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('damo', 'nlp_polylm_multialpaca_sft', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('damo', 'zh_cls_fudan-news', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('damo', 'zh_ner-JAVE', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('hjh0119', 'shareAI-Llama3-DPO-zh-en-emoji', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('huangjintao', 'AgentInstruct_copy', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', '100PoisonMpts', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', 'DocQA-RL-1.6K', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', 'MSAgent-MultiRole', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', 'MSAgent-Pro', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', 'ms_agent', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('iic', 'ms_bench', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('liucong', 'Chinese-DeepSeek-R1-Distill-data-110k-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('lmms-lab', 'multimodal-open-r1-8k-verified', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('lvjianjin', 'AdvertiseGen', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('mapjack', 'openwebtext_dataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'DuReader_robust-QG', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'MathR', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'MathR-32B-Distill', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'chinese-poetry-collection', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'clue', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'coco_2014_caption', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('modelscope', 'gsm8k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('open-r1', 'verifiable-coding-problems-python', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('open-r1', 'verifiable-coding-problems-python-10k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('open-r1', 'verifiable-coding-problems-python-10k_decontaminated', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('open-r1', 'verifiable-coding-problems-python_decontaminated', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('open-thoughts', 'OpenThoughts-114k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'self-cognition', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('sentence-transformers', 'stsb', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('shenweizhou', 'alpha-umi-toolbench-processed-v2', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('simpleai', 'HC3', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('simpleai', 'HC3-Chinese', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('speech_asr', 'speech_asr_aishell1_trainsets', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'A-OKVQA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'ChartQA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'Chinese-Qwen3-235B-2507-Distill-data-110k-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'Chinese-Qwen3-235B-Thinking-2507-Distill-data-110k-SFT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'GRIT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'GenQA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'Infinity-Instruct', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'Mantis-Instruct', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'MideficsDataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'Multimodal-Mind2Web', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'OCR-VQA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'OK-VQA_train', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'OpenHermes-2.5', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'RLAIF-V-Dataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'RedPajama-Data-1T', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'RedPajama-Data-V2', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'ScienceQA', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'SlimOrca', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'TextCaps', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'ToolBench', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'VQAv2', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'VideoChatGPT', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'WebInstructSub', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'aya_collection', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'chinese-c4', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'cinepile', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'classical_chinese_translate', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'cosmopedia-100k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'dolma', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'dolphin', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'github-code', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'gpt4v-dataset', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'llava-data', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'llava-instruct-mix-vsft', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'llava-med-zh-instruct-60k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'lnqa', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'longwriter-6k-filtered', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'medical_zh', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'moondream2-coyo-5M-captions', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'no_robots', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'orca_dpo_pairs', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'path-vqa', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'pile-val-backup', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'pixelprose', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'refcoco', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'refcocog', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'sharegpt', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'swift-sft-mixture', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'tagengo-gpt4', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'train_3.5M_CN', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'ultrachat_200k', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('swift', 'wikipedia', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('tany0699', 'garbage265', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('tastelikefeet', 'competition_math', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('wyj123456', 'GPT4all', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('wyj123456', 'code_alpaca_en', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('wyj123456', 'finance_en', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('wyj123456', 'instinwild', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('wyj123456', 'instruct', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;

INSERT INTO tag_rules (namespace, repo_name, category, tag_name, repo_type, runtime_framework, source) VALUES ('zouxuhong', 'Countdown-Tasks-3to4', 'task', 'ms-swift', 'dataset', 'ms-swift', 'ms') ON CONFLICT (namespace, repo_name, category) DO NOTHING;
