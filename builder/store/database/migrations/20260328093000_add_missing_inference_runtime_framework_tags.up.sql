SET statement_timeout = 0;

--bun:split

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name)
VALUES ('hf-inference-toolkit', 'runtime_framework', 'inference', 'model', true, 'HF Inference Toolkit')
ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name)
VALUES ('lightx2v', 'runtime_framework', 'inference', 'model', true, 'LightX2V')
ON CONFLICT ("name", category, scope) DO NOTHING;

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name)
VALUES ('audio-fish', 'runtime_framework', 'inference', 'model', true, 'Audio Fish')
ON CONFLICT ("name", category, scope) DO NOTHING;
