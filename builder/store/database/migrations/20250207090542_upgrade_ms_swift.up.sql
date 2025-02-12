SET statement_timeout = 0;

--bun:split

INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'SmallThinker-3B-Preview', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'internlm3-8b-instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Zero', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Qwen-1.5B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Qwen-7B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Qwen-14B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Qwen-32B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Llama-8B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'DeepSeek-R1-Distill-Llama-70B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'phi-4', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'MiniMax-Text-01', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'MiniCPM-V-2_6', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'MiniCPM-o-2_6', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'MiniMax-VL-01', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-7B-Instruct-1M', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-14B-Instruct-1M', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'UI-TARS-2B-SFT', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'UI-TARS-7B-SFT', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'UI-TARS-7B-DPO', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'UI-TARS-72B-SFT', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'UI-TARS-72B-DPO', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-VL-3B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-VL-7B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-VL-72B-Instruct', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Janus-Pro-1B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Janus-Pro-7B', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ms-swift', 'Qwen2.5-Math-7B-PRM800K', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
