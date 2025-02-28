SET statement_timeout = 0;

--bun:split

ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS model_format VARCHAR;

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('llama.cpp', 'runtime_framework', 'inference', 'model', true, 'Llama.cpp') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('tei', 
'runtime_framework', 'inference', 'model', true, 'TEI') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('ktransformers', 
'runtime_framework', 'inference', 'model', true, 'Ktransformers') ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'DeepSeek-R1-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'DeepSeek-V2-Lite-Chat-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'DeepSeek-V2.5-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'DeepSeek-V3-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'Mixtral-8x22B-v0.1-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'Mixtral-8x7B-Instruct-v0.1-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;
INSERT INTO resource_models (resource_name, engine_name, model_name, type) VALUES ('nvidia', 'ktransformers', 'Qwen2-57B-A14B-Instruct-GGUF', 'gpu') ON CONFLICT (engine_name, model_name) DO NOTHING;