SET statement_timeout = 0;

--bun:split
INSERT INTO public.tag_categories ("name", "scope") VALUES( 'resource', 'model') ON CONFLICT ("name", scope) DO NOTHING;
INSERT INTO public.tag_categories ("name", "scope") VALUES( 'runtime_framework', 'model') ON CONFLICT ("name", scope) DO NOTHING;

--bun:split
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('ascend', 'resource', '', 'model', true, 'Ascend') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('vllm', 'runtime_framework', 'inference', 'model', true, 'Vllm') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('tgi', 'runtime_framework', 'inference', 'model', true, 'TGI') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('mindie', 'runtime_framework', 'inference', 'model', true, 'Mindie') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('nim', 'runtime_framework', 'inference', 'model', true, 'NIM') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('llama-factory', 'runtime_framework', 'finetune', 'model', true, 'Llama factory') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name) VALUES('ms-swift', 'runtime_framework', 'finetune', 'model', true, 'Swift') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split
--for mindie
ALTER TABLE resource_models ADD CONSTRAINT unique_engine_model UNIQUE (engine_name, model_name);
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Baichuan-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Baichuan-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Baichuan2-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Baichuan2-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Bloom-176B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Bloom-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Bloomz-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'bloom', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'bloom-7b1', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'bloomz-7b1', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'ChatGLM2-6B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'ChatGLM3-6B-32K', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'CodeGeeX2-6B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'CodeLLaMA-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'CodeLLaMA-34B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'CodeLLaMA-70B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'CodeShell-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'DeepSeek-Coder-6.7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'DeepSeek-Coder-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'DeepSeek-Coder-33B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'DeepSeek-MoE-16B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Gemma-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'InterLM-20B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'InternLM2-20B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa2-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa2-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa2-70B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-33B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA3-8B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA3-70B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-65B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa-2-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa-2-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMa-2-70B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-33B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-3-8B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-3-70B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'LLaMA-65B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Mistral-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Mixtral-8x7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen-14B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen-72B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen1.5-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen1.5-14B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen1.5-32B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Qwen1.5-72B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Starcoder-15.5B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'StarCoder2-15B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Vicuna-7B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Vicuna-13B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Yi-6B-200K', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'Yi-34B-200K', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('ascend', 'mindie', 'ziya-coding-34B', 'npu') ON CONFLICT (engine_name, model_name) DO NOTHING;

--bun:split
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Mixtral-8x22B-Instruct-v0.1', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Mixtral-8x7B-Instruct-v0.1', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Mistral-7B-Instruct-v0.3', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3.1-70B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3.1-8B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3-8B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3-70B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3.1-405B-Instruct-FP8', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3.1-405b-instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3.1-8b-base', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'llama-2-13b-chat', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'llama-2-70b-chat', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'llama-2-7b-chat', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3-Taiwan-70B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama-3-Swallow-70B-Instruct-v0.1', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama3-70b-instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'nim', 'Llama3-8b-instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;




